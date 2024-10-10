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

type IKubernetesClient interface {
	GetWeightedNodes() ([]ultron.WeightedNode, error)
}

type KubernetesClient struct {
	config               *rest.Config
	mapper               *mapper.IMapper
	computeService       *services.IComputeService
	kubernetesMasterUrl  string
	kubernetesConfigPath string
}

func NewKubernetesClient(kubernetesMasterUrl string, kubernetesConfigPath string, mapper *mapper.IMapper, computeService *services.IComputeService) (*KubernetesClient, error) {
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

	return &KubernetesClient{
		config:               config,
		kubernetesMasterUrl:  kubernetesMasterUrl,
		kubernetesConfigPath: kubernetesConfigPath,
		computeService:       computeService,
		mapper:               mapper,
	}, nil
}

func (kc *KubernetesClient) GetWeightedNodes() ([]ultron.WeightedNode, error) {
	var wNodes []ultron.WeightedNode

	clientset, err := kubernetes.NewForConfig(kc.config)
	if err != nil {
		return nil, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, node := range nodes.Items {
		wNode, err := (*kc.mapper).MapNodeToWeightedNode(&node)
		if err != nil {
			return nil, err
		}

		computeConfiguration, err := (*kc.computeService).MatchWeightedNodeToComputeConfiguration(&wNode)
		if err != nil {
			return nil, err
		}

		if computeConfiguration != nil && computeConfiguration.Cost != nil && computeConfiguration.Cost.PricePerUnit != nil {
			wNode.Price = float64(*computeConfiguration.Cost.PricePerUnit)
		}

		medianPrice, err := (*kc.computeService).CalculateWeightedNodeMedianPrice(&wNode)
		if err != nil {
			return nil, err
		}

		wNode.MedianPrice = medianPrice

		interuptionRate, err := (*kc.computeService).GetInteruptionRateForWeightedNode(&wNode)
		if err != nil {
			return nil, err
		}

		wNode.InterruptionRate = *interuptionRate

		latencyRate, err := (*kc.computeService).GetLatencyRateForWeightedNode(&wNode)
		if err != nil {
			return nil, err
		}

		wNode.LatencyRate = *latencyRate

		wNodes = append(wNodes, wNode)
	}

	return wNodes, nil
}
