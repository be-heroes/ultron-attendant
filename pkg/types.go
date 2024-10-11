package pkg

type Config struct {
	RedisServerAddress   string
	RedisServerPassword  string
	RedisServerDatabase  int
	EmmaClientId         string
	EmmaClientSecret     string
	KubernetesConfigPath string
	KubernetesMasterURL  string
	CacheRefreshInterval int
}
