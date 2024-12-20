package wisp

import (
	"context"

	ultron "github.com/be-heroes/ultron/pkg"
	wisp "github.com/wispcompute/wisp-go-sdk"
)

type IWispClient interface {
	GetAllComputeConfigurations(ctx context.Context) (*[]ultron.ComputeConfiguration, error)
	GetDurableComputeConfigurations(ctx context.Context) (*[]ultron.ComputeConfiguration, error)
	GetEphemeralComputeConfigurations(ctx context.Context) (*[]ultron.ComputeConfiguration, error)
}

type WispClient struct {
	client       *wisp.APIClient
	clientId     string
	clientSecret string
}

func NewWispClient(clientId string, clientSecret string) *WispClient {
	return &WispClient{
		client:       wisp.NewAPIClient(wisp.NewConfiguration()),
		clientId:     clientId,
		clientSecret: clientSecret,
	}
}

func (wc *WispClient) GetAllComputeConfigurations(ctx context.Context) (*[]ultron.ComputeConfiguration, error) {
	// TODO: We probably need some args to map to our ConstrainRequest
	// TODO: Fetch and configure bearer token (initally it is handrolled via their portal, until we can setup a STS)
	constrainRequest := wisp.ConstrainRequest{}
	constrainResponse, _, err := wc.client.ConstraintsApi.ConstraintsCreate(ctx).ConstrainRequest(constrainRequest).Execute()
	results := []ultron.ComputeConfiguration{}

	for _, choice := range constrainResponse.GetChoice() {
		var computeType ultron.ComputeType

		if *choice.UseSpot.Get() {
			computeType = ultron.ComputeTypeEphemeral
		} else {
			computeType = ultron.ComputeTypeDurable
		}

		results = append(results, wc.mapConfiguration(&choice, computeType))
	}

	if err != nil {
		return nil, err
	}

	return &results, nil
}

func (wc *WispClient) GetDurableComputeConfigurations(ctx context.Context) (*[]ultron.ComputeConfiguration, error) {
	configurations, err := wc.GetAllComputeConfigurations(ctx)

	if err != nil {
		return nil, err
	}

	durableConfigurations := []ultron.ComputeConfiguration{}

	for _, configuration := range *configurations {
		if configuration.ComputeType == ultron.ComputeTypeDurable {
			durableConfigurations = append(durableConfigurations, configuration)
		}
	}

	return &durableConfigurations, nil
}

func (wc *WispClient) GetEphemeralComputeConfigurations(ctx context.Context) (*[]ultron.ComputeConfiguration, error) {
	configurations, err := wc.GetAllComputeConfigurations(ctx)

	if err != nil {
		return nil, err
	}

	ephemeralConfigurations := []ultron.ComputeConfiguration{}

	for _, configuration := range *configurations {
		if configuration.ComputeType == ultron.ComputeTypeEphemeral {
			ephemeralConfigurations = append(ephemeralConfigurations, configuration)
		}
	}

	return &ephemeralConfigurations, nil
}

func (ec *WispClient) mapConfiguration(clusterOffer *wisp.ClusterOffer, computeType ultron.ComputeType) ultron.ComputeConfiguration {
	provider := clusterOffer.Cloud.Get()
	location := "unknown"

	strArray, ok := clusterOffer.Regions.([]string)
	if ok {
		location = strArray[0]
	}

	cpuCount := int64(*clusterOffer.Cpus.Get())
	diskSize := int64(*clusterOffer.DiskSize.Get())
	memorySize := int64(*clusterOffer.Memory.Get())
	price := float64(*clusterOffer.Price.Get())
	priceUnit := "HOURS"
	priceCurrency := "USD"

	return ultron.ComputeConfiguration{
		Provider: provider,
		Location: &location,
		VCpu:     &cpuCount,
		RamGb:    &memorySize,
		VolumeGb: &diskSize,
		Cost: &ultron.ComputeCost{
			Unit:         &priceUnit,
			Currency:     &priceCurrency,
			PricePerUnit: &price,
		},
		ComputeType: computeType,
	}
}
