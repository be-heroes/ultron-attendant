package aws

type IAwsClient interface {
}

type AwsClient struct {
}

func NewAwsClient() (*AwsClient, error) {
	return &AwsClient{}, nil
}
