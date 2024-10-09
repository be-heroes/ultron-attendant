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

	"github.com/be-heroes/ultron-attendant/internal/kubernetes"
	ultron "github.com/be-heroes/ultron/pkg"
	services "github.com/be-heroes/ultron/pkg/services"
	emma "github.com/emma-community/emma-go-sdk"
	"github.com/redis/go-redis/v9"
)

const (
	EnvKubernetesConfig      = "KUBECONFIG"
	EnvKubernetesServiceHost = "KUBERNETES_SERVICE_HOST"
	EnvKubernetesServicePort = "KUBERNETES_SERVICE_PORT"
	EnvEmmaClientId          = "EMMA_CLIENT_ID"
	EnvEmmaClientSecret      = "EMMA_CLIENT_SECRET"
)

func main() {
	emmaApiCredentials := emma.Credentials{ClientId: os.Getenv(EnvEmmaClientId), ClientSecret: os.Getenv(EnvEmmaClientSecret)}
	emmaApiClient := emma.NewAPIClient(emma.NewConfiguration())
	kubernetesConfigPath := os.Getenv(EnvKubernetesConfig)
	kubernetesMasterUrl := fmt.Sprintf("tcp://%s:%s", os.Getenv(EnvKubernetesServiceHost), os.Getenv(EnvKubernetesServicePort))
	kubernetesClient, err := kubernetes.NewKubernetesClient(kubernetesMasterUrl, kubernetesConfigPath, nil, nil)
	if err != nil {
		log.Fatalf("Failed to create kubernetes client with error: %v", err)
	}

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
	}

	cacheService := services.NewICacheService(nil, redisClient)

	log.Println("Initializing cache")

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

	// TODO: Fetch predictions for known weighted nodes via Jarvis API
	// TODO: Fetch interuption rates for known weighted nodes via Jarvis API
	// TODO: Fetch latency rates for known weighted nodes via Jarvis API

	log.Println("Initialized cache")

	os.Exit(0)
}
