package emma

import (
	"context"
	"fmt"
	"io"
	"net/http"

	ultron "github.com/be-heroes/ultron/pkg"
	"github.com/cenkalti/backoff"
	emma "github.com/emma-community/emma-go-sdk"
)

type IEmmaClient interface {
	GetAllComputeConfigurations(ctx context.Context) (*[]ultron.ComputeConfiguration, error)
	GetDurableComputeConfigurations(ctx context.Context) (*[]ultron.ComputeConfiguration, error)
	GetEphemeralComputeConfigurations(ctx context.Context) (*[]ultron.ComputeConfiguration, error)
}

type EmmaClient struct {
	client       *emma.APIClient
	clientId     string
	clientSecret string
}

func NewEmmaClient(clientId string, clientSecret string) *EmmaClient {
	return &EmmaClient{
		client:       emma.NewAPIClient(emma.NewConfiguration()),
		clientId:     clientId,
		clientSecret: clientSecret,
	}
}

func (ec *EmmaClient) GetAllComputeConfigurations(ctx context.Context) (*[]ultron.ComputeConfiguration, error) {
	durableConfigs, err := ec.GetDurableComputeConfigurations(ctx)
	if err != nil {
		return nil, err
	}

	ephemeralConfigs, err := ec.GetEphemeralComputeConfigurations(ctx)
	if err != nil {
		return nil, err
	}

	result := append(*durableConfigs, *ephemeralConfigs...)

	return &result, err
}

func (ec *EmmaClient) GetDurableComputeConfigurations(ctx context.Context) (*[]ultron.ComputeConfiguration, error) {
	accessToken, err := ec.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	auth := context.WithValue(context.Background(), emma.ContextAccessToken, accessToken)
	durableConfigs, resp, err := ec.client.ComputeInstancesConfigurationsAPI.GetVmConfigs(auth).Execute()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("failed to fetch durable configs: %v", string(body))
	}

	var result []ultron.ComputeConfiguration

	for _, config := range durableConfigs.Content {
		result = append(result, ec.mapConfiguration(&config, ultron.ComputeTypeDurable))
	}

	return &result, nil
}

func (ec *EmmaClient) GetEphemeralComputeConfigurations(ctx context.Context) (*[]ultron.ComputeConfiguration, error) {
	accessToken, err := ec.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	auth := context.WithValue(context.Background(), emma.ContextAccessToken, accessToken)
	durableConfigs, resp, err := ec.client.ComputeInstancesConfigurationsAPI.GetVmConfigs(auth).Execute()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("failed to fetch ephemeral configs: %v", string(body))
	}

	ephemeralConfigs, resp, err := ec.client.ComputeInstancesConfigurationsAPI.GetSpotConfigs(auth).Execute()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("failed to fetch ephemeral configs: %v", string(body))
	}

	var result []ultron.ComputeConfiguration

	for _, config := range durableConfigs.Content {
		result = append(result, ec.mapConfiguration(&config, ultron.ComputeTypeDurable))
	}

	for _, config := range ephemeralConfigs.Content {
		result = append(result, ec.mapConfiguration(&config, ultron.ComputeTypeEphemeral))
	}

	return &result, nil
}

func (ec *EmmaClient) getAccessToken(ctx context.Context) (string, error) {
	credentials := emma.Credentials{ClientId: ec.clientId, ClientSecret: ec.clientSecret}
	var token string
	var err error

	operation := func() error {
		tokenResp, _, err := ec.client.AuthenticationAPI.IssueToken(ctx).Credentials(credentials).Execute()
		if err != nil {
			return err
		}

		token = tokenResp.GetAccessToken()

		return nil
	}

	backoffStrategy := backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3)

	if err = backoff.Retry(operation, backoffStrategy); err != nil {
		return "", err
	}

	return token, nil
}

func (ec *EmmaClient) mapConfiguration(config *emma.VmConfiguration, computeType ultron.ComputeType) ultron.ComputeConfiguration {
	return ultron.ComputeConfiguration{
		Identifier:        toStringPointer(config.Id),
		Provider:          config.ProviderName,
		Location:          config.LocationName,
		DataCenter:        config.DataCenterName,
		OsType:            config.OsType,
		OsVersion:         config.OsVersion,
		CloudNetworkTypes: config.CloudNetworkTypes,
		VCpuType:          config.VCpuType,
		VCpu:              toInt64Pointer(config.VCpu),
		RamGb:             toInt64Pointer(config.RamGb),
		VolumeGb:          toInt64Pointer(config.VolumeGb),
		VolumeType:        config.VolumeType,
		Cost: &ultron.ComputeCost{
			Unit:         config.Cost.Unit,
			Currency:     config.Cost.Currency,
			PricePerUnit: toFloat64Pointer(config.Cost.PricePerUnit),
		},
		ComputeType: computeType,
	}
}

func toStringPointer(value *int32) *string {
	if value == nil {
		return nil
	}

	v := string(*value)

	return &v
}

func toInt64Pointer(value *int32) *int64 {
	if value == nil {
		return nil
	}

	v := int64(*value)

	return &v
}

func toFloat64Pointer(value *float32) *float64 {
	if value == nil {
		return nil
	}

	v := float64(*value)

	return &v
}
