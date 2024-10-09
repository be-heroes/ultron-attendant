package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

type IAwsClient interface {
}

type AwsClient struct {
	config aws.Config
}

func NewAwsClient(region string) (*AwsClient, error) {
	awsConfig, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	return &AwsClient{
		config: awsConfig,
	}, nil
}
