package jarvis

// TODO: Finish Jarvis client
type IJarvisClient interface {
}

type JarvisClient struct {
}

func NewJarvisClient() (*JarvisClient, error) {
	return &JarvisClient{}, nil
}
