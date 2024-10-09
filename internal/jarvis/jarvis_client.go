package jarvis

type IJarvisClient interface {
}

type JarvisClient struct {
}

func NewJarvisClient() (*JarvisClient, error) {
	return &JarvisClient{}, nil
}
