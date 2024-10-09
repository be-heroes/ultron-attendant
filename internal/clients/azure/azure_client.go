package azure

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

type IAzureClient interface {
}

type AzureClient struct {
}

func NewAzureClient(subscriptionId string, tenantId string) (*AzureClient, error) {
	options := &azidentity.DefaultAzureCredentialOptions{
		TenantID: tenantId,
	}

	_, err := azidentity.NewDefaultAzureCredential(options)
	if err != nil {
		return nil, err
	}

	return &AzureClient{}, nil
}
