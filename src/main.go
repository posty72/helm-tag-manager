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

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	fmt.Printf("Key: %s, Secret: %s", os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"))

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
			WaitTimeSeconds:     aws.Int64(15),
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
	CommitSha     string `json:"commit_sha"`
	Repo          string `json:"repo"`
	HelmChartName string `json:"helm_chart_name"`
	Namespace     string `json:"namespace"`
}

func handleMessage(msg *sqs.Message) bool {
	fmt.Println("RECEIVING MESSAGE >>> ")
	fmt.Println(*msg.Body)

	var data Message
	if err := json.Unmarshal([]byte(*msg.Body), &data); err != nil {
		fmt.Println("failed to unmarshal:", err)
		return true
	}

	if data.HelmChartName == "" || data.CommitSha == "" || data.Repo == "" || data.Namespace == "" {
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

	patch := fmt.Sprintf(`[{"spec":{"template":{"spec":{"containers":[{"name": "%s","image":"%s:%s"}]}}}}]`, data.HelmChartName, data.Repo, data.CommitSha)

	fmt.Sprintln(patch)

	_, err = clientSet.AppsV1().Deployments(data.Namespace).Patch(context.Background(), data.HelmChartName, types.JSONPatchType, []byte(patch), v1.PatchOptions{})

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
