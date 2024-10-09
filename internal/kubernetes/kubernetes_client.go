package kubernetes

import (
	"context"
	"fmt"

	ultron "github.com/be-heroes/ultron/pkg"
	mapper "github.com/be-heroes/ultron/pkg/mapper"
	services "github.com/be-heroes/ultron/pkg/services"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesClient interface {
	GetWeightedNodes() ([]ultron.WeightedNode, error)
}

type IKubernetesClient struct {
	client               *kubernetes.Clientset
	mapper               mapper.Mapper
	computeService       services.ComputeService
	kubernetesMasterUrl  string
	kubernetesConfigPath string
}

func NewIKubernetesClient(kubernetesMasterUrl string, kubernetesConfigPath string, mapper mapper.Mapper, computeService services.ComputeService) (*IKubernetesClient, error) {
	var err error

	if kubernetesMasterUrl == "tcp://:" {
		kubernetesMasterUrl = ""
	}

	config, err := clientcmd.BuildConfigFromFlags(kubernetesMasterUrl, kubernetesConfigPath)
	if err != nil {
		fmt.Println("Falling back to docker Kubernetes API at  https://kubernetes.docker.internal:6443")

		config = &rest.Config{
			Host: "https://kubernetes.docker.internal:6443",
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: true,
			},
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &IKubernetesClient{
		client:               clientset,
		kubernetesMasterUrl:  kubernetesMasterUrl,
		kubernetesConfigPath: kubernetesConfigPath,
		computeService:       computeService,
		mapper:               mapper,
	}, nil
}

func (kc IKubernetesClient) GetWeightedNodes() ([]ultron.WeightedNode, error) {
	var wNodes []ultron.WeightedNode
	nodes, err := kc.client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, node := range nodes.Items {
		wNode, err := kc.mapper.MapNodeToWeightedNode(&node)
		if err != nil {
			return nil, err
		}

		computeConfiguration, err := kc.computeService.MatchWeightedNodeToComputeConfiguration(wNode)
		if err != nil {
			return nil, err
		}

		if computeConfiguration != nil && computeConfiguration.Cost != nil && computeConfiguration.Cost.PricePerUnit != nil {
			wNode.Price = float64(*computeConfiguration.Cost.PricePerUnit)
		}

		medianPrice, err := kc.computeService.CalculateWeightedNodeMedianPrice(wNode)
		if err != nil {
			return nil, err
		}

		wNode.MedianPrice = medianPrice

		interuptionRate, err := kc.computeService.ComputeInteruptionRateForWeightedNode(wNode)
		if err != nil {
			return nil, err
		}

		wNode.InterruptionRate = *interuptionRate

		latencyRate, err := kc.computeService.ComputeLatencyRateForWeightedNode(wNode)
		if err != nil {
			return nil, err
		}

		wNode.LatencyRate = *latencyRate

		wNodes = append(wNodes, wNode)
	}

	return wNodes, nil
}
