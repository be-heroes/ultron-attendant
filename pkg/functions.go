package pkg

import (
	"fmt"
	"os"
	"strconv"

	"github.com/be-heroes/ultron-attendant/internal/clients/kubernetes"
	ultron "github.com/be-heroes/ultron/pkg"
	"github.com/redis/go-redis/v9"
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
		KubernetesConfigPath: os.Getenv(EnvKubernetesConfig),
		CacheRefreshInterval: refreshInterval,
	}, nil
}

func InitializeRedisClient(config *Config) *redis.Client {
	if config.RedisServerAddress == "" {
		return nil
	}

	return redis.NewClient(&redis.Options{
		Addr:     config.RedisServerAddress,
		Password: os.Getenv(ultron.EnvRedisServerPassword),
		DB:       config.RedisServerDatabase,
	})
}

func InitializeKubernetesClient(config *Config) (kubernetesClient kubernetes.IKubernetesClient, err error) {
	kubernetesMasterUrl := fmt.Sprintf("tcp://%s:%s", os.Getenv(EnvKubernetesServiceHost), os.Getenv(EnvKubernetesServicePort))

	kubernetesClient, err = kubernetes.NewKubernetesClient(kubernetesMasterUrl, config.KubernetesConfigPath)
	if err != nil {
		return nil, err
	}

	return kubernetesClient, nil
}
