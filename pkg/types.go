package pkg

type Config struct {
	RedisServerAddress   string
	RedisServerDatabase  int
	EmmaClientId         string
	EmmaClientSecret     string
	KubernetesConfigPath string
	CacheRefreshInterval int
}
