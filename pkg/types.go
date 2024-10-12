package pkg

type Config struct {
	RedisServerAddress   string
	RedisServerPassword  string
	RedisServerDatabase  int
	EmmaClientId         string
	EmmaClientSecret     string
	KubernetesConfigPath string
	KubernetesMasterUrl  string
	CacheRefreshInterval int
}
