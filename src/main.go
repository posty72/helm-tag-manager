package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"helm.sh/helm/v3/pkg/chart/loader"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
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
}

func handleMessage(msg *sqs.Message) bool {
	fmt.Println("RECEIVING MESSAGE >>> ")
	fmt.Println(*msg.Body)

	var data Message
	if err := json.Unmarshal([]byte(*msg.Body), &data); err != nil {
		fmt.Println("failed to unmarshal:", err)
		return true
	}

	kubeconfigPath := "~/kubeconfig"
	releaseNamespace := "default"
	actionConfig := new(action.Configuration)
	err := actionConfig.Init(kube.GetConfig(kubeconfigPath, "", releaseNamespace), releaseNamespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Sprintf(format, v)
	})

	if err != nil {
		panic(err)
	}

	fmt.Println(data.HelmChartName)
	fmt.Println(data.CommitSha)
	fmt.Println(data.RepoName)

	chartPath := "/tmp/my-chart-0.1.0.tgz"
	chart, err := loader.Load(chartPath)
	if err != nil {
		panic(err)
	}

	iCli := action.NewUpgrade(actionConfig)

	values := map[string]interface{}{
		"image.tag": data.CommitSha,
	}

	iCli.Namespace = releaseNamespace
	rel, err := iCli.Run(data.HelmChartName, chart, values)
	if err != nil {
		panic(err)
	}

	fmt.Println("Successfully installed release: ", rel.Name)

	return true
}

func deleteMessage(queueURL string, msg *sqs.Message) {
	sqsSvc.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queueURL),
		ReceiptHandle: msg.ReceiptHandle,
	})
}
