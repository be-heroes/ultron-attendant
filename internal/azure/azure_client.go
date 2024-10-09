package azure

type IAzureClient interface {
}

type AzureClient struct {
}

func NewAzureClient() (*AzureClient, error) {
	return &AzureClient{}, nil
}
