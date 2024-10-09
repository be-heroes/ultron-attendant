package gcp

type IGcpClient interface {
}

type GcpClient struct {
}

func NewGcpClient() (*GcpClient, error) {
	return &GcpClient{}, nil
}
