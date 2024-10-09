package gcp

type IGcpClient interface {
}

type GcpClient struct {
}

func NewGCPClient() (*GcpClient, error) {
	return &GcpClient{}, nil
}
