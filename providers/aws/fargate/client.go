package fargate

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
)

// Client communicates with the regional AWS Fargate service.
type Client struct {
	region  string
	svc     *ecs.ECS
	api     ecsiface.ECSAPI
	logsapi cloudwatchlogsiface.CloudWatchLogsAPI
}

var client *Client

// NewClient creates a new Fargate client in the given region.
func newClient(region string) (*Client, error) {
	var client Client

	// Initialize client session configuration.
	config := aws.NewConfig()
	config.Region = aws.String(region)

	session, err := session.NewSessionWithOptions(
		session.Options{
			Config:            *config,
			SharedConfigState: session.SharedConfigEnable,
		},
	)

	if err != nil {
		return nil, err
	}

	// Create the Fargate service client.
	client.region = region
	client.svc = ecs.New(session)
	client.api = client.svc

	// Create the CloudWatch service client.
	client.logsapi = cloudwatchlogs.New(session)

	log.Println("Created Fargate service client.")

	return &client, nil
}
