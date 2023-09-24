package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	sqsSvc *sqs.SQS
)

func main() {
	queueName := os.Getenv("QUEUE_NAME")
	queue := flag.String("q", queueName, "The name of the queue")
	timeout := flag.Int64("t", 5, "How long, in seconds, that the message is hidden from others")
	flag.Parse()

	if *queue == "" {
		fmt.Println("You must supply the name of a queue (-q QUEUE)")
		return
	}

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("ap-southeast-2"),
		Credentials: credentials.NewStaticCredentials(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""),
	})
	if err != nil {
		fmt.Println("Error", err)
		return
	}

	if *queue == "" {
		fmt.Println("You must supply the name of a queue (-q QUEUE)")
		return
	}

	if *timeout < 0 {
		*timeout = 0
	}

	if *timeout > 12*60*60 {
		*timeout = 12 * 60 * 60
	}

	sqsSvc = sqs.New(sess)

	urlResult, err := sqsSvc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: queue,
	})

	if err != nil {
		fmt.Println("Error", err)
		return
	}

	queueURL := urlResult.QueueUrl

	chnMessages := make(chan *sqs.Message, 2)
	go pollMessages(*queueURL, chnMessages)

	for message := range chnMessages {
		delete := handleMessage(message)

		if delete {
			deleteMessage(*queueURL, message)
		}
	}
}

func pollMessages(queueURL string, chn chan<- *sqs.Message) {

	for {
		output, err := sqsSvc.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: aws.Int64(2),
			WaitTimeSeconds:     aws.Int64(25),
		})

		if err != nil {
			fmt.Println("failed to fetch sqs message %v", err)
		}

		for _, message := range output.Messages {
			chn <- message
		}

	}

}

type Message struct {
	ImageTag       string `json:"image_tag"`
	Repo           string `json:"repo"`
	DeploymentName string `json:"deployment_name"`
	ContainerName  string `json:"container_name"`
	Namespace      string `json:"namespace"`
}

func handleMessage(msg *sqs.Message) bool {
	fmt.Println("RECEIVING MESSAGE >>> ")
	fmt.Println(*msg.Body)

	var data Message
	if err := json.Unmarshal([]byte(*msg.Body), &data); err != nil {
		fmt.Println("failed to unmarshal:", err)
		return true
	}

	if data.ContainerName == "" || data.ImageTag == "" || data.Repo == "" || data.Namespace == "" || data.DeploymentName == "" {
		fmt.Println("missing data")
		return true
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{CurrentContext: ""}).ClientConfig()
	if err != nil {
		fmt.Println("failed to create config", err)
		return false
	}

	clientSet, err := kubernetes.NewForConfig(config)
	deploymentClient := clientSet.AppsV1().Deployments(data.Namespace)

	result, err := deploymentClient.Get(context.TODO(), data.DeploymentName, metav1.GetOptions{})
	if err != nil {
		fmt.Println("could not find deployment", err)
		return true
	}

	containerIndex := -1
	for i := 0; i < len(result.Spec.Template.Spec.Containers); i++ {
		if result.Spec.Template.Spec.Containers[i].Name == data.ContainerName {
			containerIndex = i
			break
		}
	}
	if containerIndex == -1 {
		fmt.Println("could not find container")
		return true
	}

	result.Spec.Template.Spec.Containers[containerIndex].Image = data.Repo + ":" + data.ImageTag

	_, err = deploymentClient.Update(context.TODO(), result, metav1.UpdateOptions{})
	if err != nil {
		fmt.Println("failed to patch deployment", err)
		return true
	}

	println("Successfully patched deployment")

	return true
}

func deleteMessage(queueURL string, msg *sqs.Message) {
	println("DELETING MESSAGE >>>")
	sqsSvc.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queueURL),
		ReceiptHandle: msg.ReceiptHandle,
	})
}
