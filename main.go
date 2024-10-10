package main

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/be-heroes/ultron-attendant/internal/clients/kubernetes"
	attendant "github.com/be-heroes/ultron-attendant/pkg"
	ultron "github.com/be-heroes/ultron/pkg"
	algorithm "github.com/be-heroes/ultron/pkg/algorithm"
	mapper "github.com/be-heroes/ultron/pkg/mapper"
	services "github.com/be-heroes/ultron/pkg/services"
	emma "github.com/emma-community/emma-go-sdk"
)

type Config struct {
	RedisServerAddress   string
	RedisServerDatabase  int
	EmmaClientId         string
	EmmaClientSecret     string
	KubernetesConfigPath string
	CacheRefreshInterval int
}

func LoadConfig() (*Config, error) {
	redisDatabase, err := strconv.Atoi(os.Getenv(ultron.EnvRedisServerDatabase))
	if err != nil {
		redisDatabase = 0
	}
	refreshInterval, err := strconv.Atoi(os.Getenv(attendant.EnvCacheRefreshInterval))
	if err != nil {
		refreshInterval = 15
	}

	return &Config{
		RedisServerAddress:   os.Getenv(ultron.EnvRedisServerAddress),
		RedisServerDatabase:  redisDatabase,
		EmmaClientId:         os.Getenv(attendant.EnvEmmaClientId),
		EmmaClientSecret:     os.Getenv(attendant.EnvEmmaClientSecret),
		KubernetesConfigPath: os.Getenv(attendant.EnvKubernetesConfig),
		CacheRefreshInterval: refreshInterval,
	}, nil
}

func initializeRedis(config *Config) *redis.Client {
	if config.RedisServerAddress == "" {
		return nil
	}
	return redis.NewClient(&redis.Options{
		Addr:     config.RedisServerAddress,
		Password: os.Getenv(ultron.EnvRedisServerPassword),
		DB:       config.RedisServerDatabase,
	})
}

func initializeKubernetesClient(config *Config, mapper mapper.IMapper, computeService services.IComputeService) (*kubernetes.KubernetesClient, error) {
	kubernetesMasterUrl := fmt.Sprintf("tcp://%s:%s", os.Getenv(attendant.EnvKubernetesServiceHost), os.Getenv(attendant.EnvKubernetesServicePort))
	return kubernetes.NewKubernetesClient(kubernetesMasterUrl, config.KubernetesConfigPath, mapper, computeService)
}

func startCacheRefreshLoop(ctx context.Context, logger *zap.SugaredLogger, emmaApiClient *emma.APIClient, config *Config, cacheService services.ICacheService, kubernetesClient *kubernetes.KubernetesClient) {
	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutting down cache refresh loop")
			return
		default:
			logger.Info("Refreshing cache")
			refreshCache(ctx, logger, emmaApiClient, config, cacheService, kubernetesClient)
			time.Sleep(time.Duration(config.CacheRefreshInterval) * time.Minute)
		}
	}
}

func refreshCache(ctx context.Context, logger *zap.SugaredLogger, emmaApiClient *emma.APIClient, config *Config, cacheService services.ICacheService, kubernetesClient *kubernetes.KubernetesClient) {
	results := make(chan error, 3)
	emmaAuth := context.WithValue(ctx, emma.ContextAccessToken, getEmmaAccessToken(ctx, logger, emmaApiClient, config))

	// Concurrently fetch configurations
	go func() {
		_, resp, err := emmaApiClient.ComputeInstancesConfigurationsAPI.GetVmConfigs(emmaAuth).Size(math.MaxInt32).Execute()
		if err != nil || resp.StatusCode != http.StatusOK {
			results <- fmt.Errorf("failed to fetch durable configs: %v", err)
		} else {
			cacheService.AddCacheItem(ultron.CacheKeyDurableVmConfigurations, nil, 0) // Add the actual data
			results <- nil
		}
	}()

	go func() {
		_, resp, err := emmaApiClient.ComputeInstancesConfigurationsAPI.GetSpotConfigs(emmaAuth).Size(math.MaxInt32).Execute()
		if err != nil || resp.StatusCode != http.StatusOK {
			results <- fmt.Errorf("failed to fetch spot configs: %v", err)
		} else {
			cacheService.AddCacheItem(ultron.CacheKeySpotVmConfigurations, nil, 0) // Add the actual data
			results <- nil
		}
	}()

	go func() {
		wNodes, err := kubernetesClient.GetWeightedNodes()
		if err != nil {
			results <- fmt.Errorf("failed to get weighted nodes: %v", err)
		} else {
			cacheService.AddCacheItem(ultron.CacheKeyWeightedNodes, wNodes, 0)
			results <- nil
		}
	}()

	for i := 0; i < 3; i++ {
		if err := <-results; err != nil {
			logger.Warnw("Error during cache refresh", "error", err)
		}
	}

	logger.Info("Cache refresh complete")
}

func getEmmaAccessToken(ctx context.Context, logger *zap.SugaredLogger, emmaApiClient *emma.APIClient, config *Config) string {
	credentials := emma.Credentials{ClientId: config.EmmaClientId, ClientSecret: config.EmmaClientSecret}
	var token string
	var err error
	operation := func() error {
		tokenResp, _, err := emmaApiClient.AuthenticationAPI.IssueToken(ctx).Credentials(credentials).Execute()
		if err != nil {
			return err
		}
		token = tokenResp.GetAccessToken()
		return nil
	}
	backoffStrategy := backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3)
	if err = backoff.Retry(operation, backoffStrategy); err != nil {
		logger.Fatalw("Failed to obtain Emma API access token", "error", err)
	}
	return token
}

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()
	sugar.Info("Initializing ultron-attendant")

	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		sugar.Fatalw("Failed to load configuration", "error", err)
	}

	// Initialize Redis client
	redisClient := initializeRedis(config)
	if redisClient != nil {
		if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
			sugar.Fatalw("Failed to connect to Redis", "error", err)
		}
	}

	// Initialize Mapper and Compute Service
	mapperInstance := mapper.NewMapper()
	algorithmInstance := algorithm.NewAlgorithm()
	cacheService := services.NewCacheService(nil, redisClient)
	computeService := services.NewComputeService(algorithmInstance, cacheService, mapperInstance)

	// Initialize Kubernetes client
	kubernetesClient, err := initializeKubernetesClient(config, mapperInstance, computeService)
	if err != nil {
		sugar.Fatalw("Failed to initialize Kubernetes client", "error", err)
	}

	// Initialize Emma API client
	emmaApiClient := emma.NewAPIClient(emma.NewConfiguration())

	// Set up graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start the cache refresh loop
	go startCacheRefreshLoop(ctx, sugar, emmaApiClient, config, cacheService, kubernetesClient)

	// Wait for shutdown signal
	<-ctx.Done()
	sugar.Info("Shutdown signal received, cleaning up...")
	stop()
	sugar.Info("Ultron-attendant shut down gracefully")
}
