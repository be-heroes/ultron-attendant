package azure

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

type IAzureClient interface {
}

type AzureClient struct {
	credentials *azidentity.DefaultAzureCredential
}

func NewAzureClient(tenantId string) (*AzureClient, error) {
	options := &azidentity.DefaultAzureCredentialOptions{
		TenantID: tenantId,
	}

	credentials, err := azidentity.NewDefaultAzureCredential(options)
	if err != nil {
		return nil, err
	}

	return &AzureClient{
		credentials: credentials,
	}, nil
}
