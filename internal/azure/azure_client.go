package azure

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

type IAzureClient interface {
}

type AzureClient struct {
}

func NewAzureClient(subscriptionId string) (*AzureClient, error) {
	_, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}

	return &AzureClient{}, nil
}
