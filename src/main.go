package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
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
	queue := flag.String("q", "helm_tag_manager_queue.fifo", "The name of the queue")
	timeout := flag.Int64("t", 5, "How long, in seconds, that the message is hidden from others")
	flag.Parse()

	if *queue == "" {
		fmt.Println("You must supply the name of a queue (-q QUEUE)")
		return
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

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
	RepoName      string `json:"repo_name"`
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

	fmt.Println(data.HelmChartName)
	fmt.Println(data.CommitSha)
	fmt.Println(data.RepoName)

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{CurrentContext: ""}).ClientConfig()
	if err != nil {
		panic(err.Error())
	}

	clientSet, err := kubernetes.NewForConfig(config)

	patch := []byte(fmt.Sprintf(`[{"spec":{"template":{"spec":{"containers":[{"name": "spend-webapp","image":"708991919921.dkr.ecr.ap-southeast-2.amazonaws.com/spend-webapp:%s"}]}}}}]`, data.CommitSha))

	_, err = clientSet.AppsV1().Deployments("spend").Patch(context.Background(), data.HelmChartName, types.JSONPatchType, patch, v1.PatchOptions{})

	if err != nil {
		panic(err.Error())
	}

	println("Successfully patched deployment")

	return true
}

func deleteMessage(queueURL string, msg *sqs.Message) {
	sqsSvc.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queueURL),
		ReceiptHandle: msg.ReceiptHandle,
	})
}
