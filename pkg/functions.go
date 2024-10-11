package pkg

import (
	"fmt"
	"os"
	"strconv"

	ultron "github.com/be-heroes/ultron/pkg"
	services "github.com/be-heroes/ultron/pkg/services"
)

func LoadConfig() (*Config, error) {
	redisDatabase, err := strconv.Atoi(os.Getenv(ultron.EnvRedisServerDatabase))
	if err != nil {
		redisDatabase = 0
	}

	refreshInterval, err := strconv.Atoi(os.Getenv(EnvCacheRefreshInterval))
	if err != nil {
		refreshInterval = 15
	}

	return &Config{
		RedisServerAddress:   os.Getenv(ultron.EnvRedisServerAddress),
		RedisServerDatabase:  redisDatabase,
		EmmaClientId:         os.Getenv(EnvEmmaClientId),
		EmmaClientSecret:     os.Getenv(EnvEmmaClientSecret),
		KubernetesConfigPath: os.Getenv(ultron.EnvKubernetesConfig),
		KubernetesMasterURL:  fmt.Sprintf("tcp://%s:%s", os.Getenv(ultron.EnvKubernetesServiceHost), os.Getenv(ultron.EnvKubernetesServicePort)),
		CacheRefreshInterval: refreshInterval,
	}, nil
}

func InitializeKubernetesServiceFromConfig(config *Config) (kubernetesService services.IKubernetesService, err error) {
	kubernetesService, err = services.NewKubernetesClient(config.KubernetesMasterURL, config.KubernetesConfigPath)
	if err != nil {
		return nil, err
	}

	return kubernetesService, nil
}