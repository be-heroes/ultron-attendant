package main

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/cenkalti/backoff"
	"go.uber.org/zap"

	"github.com/be-heroes/ultron-attendant/internal/clients/kubernetes"
	attendant "github.com/be-heroes/ultron-attendant/pkg"
	ultron "github.com/be-heroes/ultron/pkg"
	algorithm "github.com/be-heroes/ultron/pkg/algorithm"
	mapper "github.com/be-heroes/ultron/pkg/mapper"
	services "github.com/be-heroes/ultron/pkg/services"
	emma "github.com/emma-community/emma-go-sdk"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	sugar := logger.Sugar()
	sugar.Info("Initializing ultron-attendant")

	config, err := attendant.LoadConfig()
	if err != nil {
		sugar.Fatalw("Failed to load configuration", "error", err)
	}

	redisClient := attendant.InitializeRedisClient(config)
	if redisClient != nil {
		if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
			sugar.Fatalw("Failed to connect to Redis", "error", err)
		}
	}

	mapperInstance := mapper.NewMapper()
	algorithmInstance := algorithm.NewAlgorithm()
	cacheService := services.NewCacheService(nil, redisClient)
	computeService := services.NewComputeService(algorithmInstance, cacheService, mapperInstance)
	kubernetesClient, err := attendant.InitializeKubernetesClient(config)
	if err != nil {
		sugar.Fatalw("Failed to initialize Kubernetes client", "error", err)
	}

	emmaApiClient := emma.NewAPIClient(emma.NewConfiguration())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go startCacheRefreshLoop(ctx, sugar, emmaApiClient, config, cacheService, kubernetesClient, computeService, mapperInstance)

	<-ctx.Done()

	sugar.Info("Shutdown signal received, cleaning up...")

	stop()

	sugar.Info("Ultron-attendant shut down gracefully")
}

func startCacheRefreshLoop(ctx context.Context, logger *zap.SugaredLogger, emmaApiClient *emma.APIClient, config *attendant.Config, cacheService services.ICacheService, kubernetesClient kubernetes.IKubernetesClient, computeService services.IComputeService, mapper mapper.IMapper) {
	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutting down cache refresh loop")

			return
		default:
			logger.Info("Refreshing cache")

			refreshCache(ctx, logger, emmaApiClient, config, cacheService, kubernetesClient, computeService, mapper)

			time.Sleep(time.Duration(config.CacheRefreshInterval) * time.Minute)
		}
	}
}

func refreshCache(ctx context.Context, logger *zap.SugaredLogger, emmaApiClient *emma.APIClient, config *attendant.Config, cacheService services.ICacheService, kubernetesClient kubernetes.IKubernetesClient, computeService services.IComputeService, mapper mapper.IMapper) {
	results := make(chan error, 3)
	emmaAuth := context.WithValue(ctx, emma.ContextAccessToken, getEmmaAccessToken(ctx, logger, emmaApiClient, config))

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
		nodes, err := kubernetesClient.GetNodes()
		if err != nil {
			results <- fmt.Errorf("failed to get nodes: %v", err)
		} else {
			var wNodes []ultron.WeightedNode

			for _, node := range nodes {
				wNode, err := mapper.MapNodeToWeightedNode(&node)
				if err != nil {
					results <- fmt.Errorf("failed to map to weighted nodes: %v", err)
				}

				computeConfiguration, err := computeService.MatchWeightedNodeToComputeConfiguration(&wNode)
				if err != nil {
					results <- fmt.Errorf("failed to match compute configuration: %v", err)
				}

				if computeConfiguration != nil && computeConfiguration.Cost != nil && computeConfiguration.Cost.PricePerUnit != nil {
					wNode.Price = float64(*computeConfiguration.Cost.PricePerUnit)
				}

				medianPrice, err := computeService.CalculateWeightedNodeMedianPrice(&wNode)
				if err != nil {
					results <- fmt.Errorf("failed to calculate median price: %v", err)
				}

				wNode.MedianPrice = medianPrice

				interuptionRate, err := computeService.GetInteruptionRateForWeightedNode(&wNode)
				if err != nil {
					results <- fmt.Errorf("failed to get interuption rate for weighted node: %v", err)
				}

				wNode.InterruptionRate = *interuptionRate

				latencyRate, err := computeService.GetLatencyRateForWeightedNode(&wNode)
				if err != nil {
					results <- fmt.Errorf("failed to get latency rate for weighted node: %v", err)
				}

				wNode.LatencyRate = *latencyRate

				wNodes = append(wNodes, wNode)
			}

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

func getEmmaAccessToken(ctx context.Context, logger *zap.SugaredLogger, emmaApiClient *emma.APIClient, config *attendant.Config) string {
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
