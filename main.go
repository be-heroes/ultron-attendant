package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	emma "github.com/be-heroes/ultron-attendant/internal/clients/emma"
	attendant "github.com/be-heroes/ultron-attendant/pkg"
	ultron "github.com/be-heroes/ultron/pkg"
	algorithm "github.com/be-heroes/ultron/pkg/algorithm"
	mapper "github.com/be-heroes/ultron/pkg/mapper"
	services "github.com/be-heroes/ultron/pkg/services"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	redisClient := ultron.InitializeRedisClient(config.RedisServerAddress, config.RedisServerPassword, config.RedisServerDatabase)
	if redisClient != nil {
		if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
			sugar.Fatalw("Failed to connect to Redis", "error", err)
		}
	}

	mapperInstance := mapper.NewMapper()
	algorithmInstance := algorithm.NewAlgorithm()
	cacheService := services.NewCacheService(nil, redisClient)
	computeService := services.NewComputeService(algorithmInstance, cacheService, mapperInstance)
	kubernetesClient, err := attendant.InitializeKubernetesServiceFromConfig(config)
	emmaClient := emma.NewEmmaClient(config.EmmaClientId, config.EmmaClientSecret)
	if err != nil {
		sugar.Fatalw("Failed to initialize Kubernetes client", "error", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go startCacheRefreshLoop(ctx, sugar, emmaClient, config, cacheService, kubernetesClient, computeService, mapperInstance)

	<-ctx.Done()

	sugar.Info("Shutdown signal received, cleaning up...")

	stop()

	sugar.Info("Ultron-attendant shut down gracefully")
}

func startCacheRefreshLoop(ctx context.Context, logger *zap.SugaredLogger, emmaClient *emma.EmmaClient, config *attendant.Config, cacheService services.ICacheService, kubernetesService services.IKubernetesService, computeService services.IComputeService, mapper mapper.IMapper) {
	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutting down cache refresh loop")

			return
		default:
			logger.Info("Refreshing cache")

			refreshCache(ctx, logger, emmaClient, config, cacheService, kubernetesService, computeService, mapper)

			time.Sleep(time.Duration(config.CacheRefreshInterval) * time.Minute)
		}
	}
}

func refreshCache(ctx context.Context, logger *zap.SugaredLogger, emmaClient *emma.EmmaClient, config *attendant.Config, cacheService services.ICacheService, kubernetesService services.IKubernetesService, computeService services.IComputeService, mapper mapper.IMapper) {
	results := make(chan error, 3)

	go func() {
		durableConfigs, err := emmaClient.GetDurableComputeConfigurations(ctx)
		if err != nil {
			results <- fmt.Errorf("failed to fetch durable configs: %v", err)
		} else {
			cacheService.AddCacheItem(ultron.CacheKeyDurableComputeConfigurations, durableConfigs, 0)
			results <- nil
		}
	}()

	go func() {
		ephemeralConfigs, err := emmaClient.GetEphemeralComputeConfigurations(ctx)
		if err != nil {
			results <- fmt.Errorf("failed to fetch ephemeral configs: %v", err)
		} else {
			cacheService.AddCacheItem(ultron.CacheKeyEphemeralComputeConfigurations, ephemeralConfigs, 0)
			results <- nil
		}
	}()

	go func() {
		nodes, err := kubernetesService.GetNodes(context.Background(), metav1.ListOptions{})
		if err != nil {
			results <- err
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
