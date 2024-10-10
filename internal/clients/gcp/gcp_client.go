package gcp

import "os"

// TODO: Finish Gcp client
type IGcpClient interface {
}

type GcpClient struct {
	credentials string
}

func NewGcpClient() (*GcpClient, error) {
	return &GcpClient{
		credentials: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
	}, nil
}
