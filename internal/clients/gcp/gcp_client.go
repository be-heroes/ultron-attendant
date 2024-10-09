package gcp

import "os"

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
