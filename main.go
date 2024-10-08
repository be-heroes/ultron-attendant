package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"

	"github.com/be-heroes/ultron-attendant/internal/kubernetes"
	ultron "github.com/be-heroes/ultron/pkg"
	services "github.com/be-heroes/ultron/pkg/services"
	emma "github.com/emma-community/emma-go-sdk"
)

const (
	EnvironmentVariableKeyKubernetesConfig      = "KUBECONFIG"
	EnvironmentVariableKeyKubernetesServiceHost = "KUBERNETES_SERVICE_HOST"
	EnvironmentVariableKeyKubernetesServicePort = "KUBERNETES_SERVICE_PORT"
	EnvironmentVariableKeyEmmaClientId          = "EMMA_CLIENT_ID"
	EnvironmentVariableKeyEmmaClientSecret      = "EMMA_CLIENT_SECRET"
)

func main() {
	kubernetesConfigPath := os.Getenv(EnvironmentVariableKeyKubernetesConfig)
	kubernetesMasterUrl := fmt.Sprintf("tcp://%s:%s", os.Getenv(EnvironmentVariableKeyKubernetesServiceHost), os.Getenv(EnvironmentVariableKeyKubernetesServicePort))
	kubernetesClient := kubernetes.NewIKubernetesClient(kubernetesMasterUrl, kubernetesConfigPath, nil, nil)
	emmaApiCredentials := emma.Credentials{ClientId: os.Getenv(EnvironmentVariableKeyEmmaClientId), ClientSecret: os.Getenv(EnvironmentVariableKeyEmmaClientSecret)}
	apiClient := emma.NewAPIClient(emma.NewConfiguration())

	// TODO: Initialize redisClient and pass to cacheService
	cacheService := services.NewICacheService(nil, nil)

	log.Println("Initializing cache")

	token, resp, err := apiClient.AuthenticationAPI.IssueToken(context.Background()).Credentials(emmaApiCredentials).Execute()
	if err != nil {
		log.Fatalf("Failed to issue access token with error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		_, err := io.ReadAll(resp.Body)

		log.Fatalf("Failed to read access token data with error: %v", err)
	}

	auth := context.WithValue(context.Background(), emma.ContextAccessToken, token.GetAccessToken())
	durableConfigs, resp, err := apiClient.ComputeInstancesConfigurationsAPI.GetVmConfigs(auth).Size(math.MaxInt32).Execute()

	log.Printf("durableConfigs: %v", durableConfigs)

	if err != nil {
		log.Fatalf("Failed to fetch durable compute configurations with error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		_, err := io.ReadAll(resp.Body)

		log.Fatalf("Failed to read durable compute configurations data with error: %v", err)
	}

	ephemeralConfigs, resp, err := apiClient.ComputeInstancesConfigurationsAPI.GetSpotConfigs(auth).Size(math.MaxInt32).Execute()

	log.Printf("ephemeralConfigs: %v", ephemeralConfigs)

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
}
