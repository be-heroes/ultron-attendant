package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/be-heroes/ultron-attendant/internal/clients/kubernetes"
	attendant "github.com/be-heroes/ultron-attendant/pkg"
	ultron "github.com/be-heroes/ultron/pkg"
	algorithm "github.com/be-heroes/ultron/pkg/algorithm"
	mapper "github.com/be-heroes/ultron/pkg/mapper"
	services "github.com/be-heroes/ultron/pkg/services"
	emma "github.com/emma-community/emma-go-sdk"
	"github.com/redis/go-redis/v9"
)

func main() {
	log.Println("Initializing ultron-attendant")

	var redisClient *redis.Client

	redisServerAddress := os.Getenv(ultron.EnvRedisServerAddress)
	redisServerDatabase := os.Getenv(ultron.EnvRedisServerDatabase)
	redisServerDatabaseInt, err := strconv.Atoi(redisServerDatabase)
	if err != nil {
		redisServerDatabaseInt = 0
	}

	if redisServerAddress != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     redisServerAddress,
			Password: os.Getenv(ultron.EnvRedisServerPassword),
			DB:       redisServerDatabaseInt,
		})

		_, err := redisClient.Ping(context.Background()).Result()
		if err != nil {
			log.Fatalf("Failed to ping redis server with error: %v", err)
		}
	}

	var mapper mapper.IMapper = mapper.NewMapper()
	var algorithm algorithm.IAlgorithm = algorithm.NewAlgorithm()
	var cacheService services.ICacheService = services.NewCacheService(nil, redisClient)
	var computeService services.IComputeService = services.NewComputeService(algorithm, cacheService, mapper)
	emmaApiCredentials := emma.Credentials{ClientId: os.Getenv(attendant.EnvEmmaClientId), ClientSecret: os.Getenv(attendant.EnvEmmaClientSecret)}
	emmaApiClient := emma.NewAPIClient(emma.NewConfiguration())
	kubernetesConfigPath := os.Getenv(attendant.EnvKubernetesConfig)
	kubernetesMasterUrl := fmt.Sprintf("tcp://%s:%s", os.Getenv(attendant.EnvKubernetesServiceHost), os.Getenv(attendant.EnvKubernetesServicePort))
	kubernetesClient, err := kubernetes.NewKubernetesClient(kubernetesMasterUrl, kubernetesConfigPath, mapper, computeService)
	if err != nil {
		log.Fatalf("Failed to create kubernetes client with error: %v", err)
	}

	cacheRefreshInterval := os.Getenv(attendant.EnvCacheRefreshInterval)
	cacheRefreshIntervalInt, err := strconv.Atoi(cacheRefreshInterval)
	if err != nil {
		cacheRefreshIntervalInt = 15
	}

	log.Println("Initialized ultron-attendant")
	log.Println("Starting ultron-attendant")

	for {
		log.Println("Refreshing cache")

		token, resp, err := emmaApiClient.AuthenticationAPI.IssueToken(context.Background()).Credentials(emmaApiCredentials).Execute()
		if err != nil {
			log.Fatalf("Failed to issue access token with error: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			_, err := io.ReadAll(resp.Body)

			log.Fatalf("Failed to read access token data with error: %v", err)
		}

		auth := context.WithValue(context.Background(), emma.ContextAccessToken, token.GetAccessToken())
		durableConfigs, resp, err := emmaApiClient.ComputeInstancesConfigurationsAPI.GetVmConfigs(auth).Size(math.MaxInt32).Execute()

		if err != nil {
			log.Fatalf("Failed to fetch durable compute configurations with error: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			_, err := io.ReadAll(resp.Body)

			log.Fatalf("Failed to read durable compute configurations data with error: %v", err)
		}

		ephemeralConfigs, resp, err := emmaApiClient.ComputeInstancesConfigurationsAPI.GetSpotConfigs(auth).Size(math.MaxInt32).Execute()

		if err != nil {
			log.Fatalf("Failed to fetch ephemeral compute configurations with error: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			_, err := io.ReadAll(resp.Body)

			log.Fatalf("Failed to read ephemeral compute configurations data with error: %v", err)
		}

		cacheService.AddCacheItem(ultron.CacheKeyDurableVmConfigurations, durableConfigs.Content, 0)
		cacheService.AddCacheItem(ultron.CacheKeySpotVmConfigurations, ephemeralConfigs.Content, 0)

		wNodes, err := kubernetesClient.GetWeightedNodes()
		if err != nil {
			log.Fatalf("Failed to get weighted nodes with error: %v", err)
		}

		cacheService.AddCacheItem(ultron.CacheKeyWeightedNodes, wNodes, 0)

		// TODO: Generate Jarvis Golang SDK once OpenAPI contract is finalized
		// TODO: Fetch predictions for known weighted nodes via Jarvis API
		// TODO: Fetch interuption rates for known weighted nodes via Jarvis API
		// TODO: Fetch latency rates for known weighted nodes via Jarvis API

		log.Println("Refreshed cache")

		time.Sleep(time.Duration(cacheRefreshIntervalInt) * time.Minute)
	}
}
