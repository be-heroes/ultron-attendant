package emma

import (
	wisp "github.com/wispcompute/wisp-go-sdk"
)

type IWispClient interface {
}

type WispClient struct {
	client       *wisp.APIClient
	clientId     string
	clientSecret string
}

func NewEmmaClient(clientId string, clientSecret string) *WispClient {
	return &WispClient{
		client:       wisp.NewAPIClient(wisp.NewConfiguration()),
		clientId:     clientId,
		clientSecret: clientSecret,
	}
}
