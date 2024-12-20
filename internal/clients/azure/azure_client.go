package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	ultron "github.com/be-heroes/ultron/pkg"
)

type AzureComputePricesResponse struct {
	Items        []AzureComputePrice `json:"Items"`
	Count        int                 `json:"Count"`
	NextPageLink string              `json:"NextPageLink"`
}

type AzureComputePrice struct {
	CurrencyCode         string  `json:"currencyCode"`
	TierMinimumUnits     float64 `json:"tierMinimumUnits"`
	RetailPrice          float64 `json:"retailPrice"`
	UnitPrice            float64 `json:"unitPrice"`
	ArmRegionName        string  `json:"armRegionName"`
	Location             string  `json:"location"`
	EffectiveStartDate   string  `json:"effectiveStartDate"`
	MeterID              string  `json:"meterId"`
	MeterName            string  `json:"meterName"`
	ProductID            string  `json:"productId"`
	SkuID                string  `json:"skuId"`
	ProductName          string  `json:"productName"`
	SkuName              string  `json:"skuName"`
	ServiceName          string  `json:"serviceName"`
	ServiceID            string  `json:"serviceId"`
	ServiceFamily        string  `json:"serviceFamily"`
	UnitOfMeasure        string  `json:"unitOfMeasure"`
	Type                 string  `json:"type"`
	IsPrimaryMeterRegion bool    `json:"isPrimaryMeterRegion"`
	ArmSkuName           string  `json:"armSkuName"`
}

type IAzureClient interface {
	GetComputeCost(ctx context.Context, filter string) (*[]ultron.ComputeCost, error)
}

// TODO: Refactor client to return compute configs, as well as compute costs and adhere to the same interface as the emma & wisp clients
type AzureClient struct {
	httpClient *http.Client
	baseUrl    string
}

func NewAzureClient(httpClient *http.Client, baseUrl string) *AzureClient {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &AzureClient{
		httpClient: httpClient,
		baseUrl:    baseUrl,
	}
}

func (c *AzureClient) GetComputeCost(ctx context.Context, filter string) (*[]ultron.ComputeCost, error) {
	var allItems []ultron.ComputeCost

	u, err := url.Parse(c.baseUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %v", err)
	}

	q := u.Query()
	if filter != "" {
		q.Set("$filter", filter)
	}
	u.RawQuery = q.Encode()

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %v", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("non-OK HTTP status: %s", resp.Status)
		}

		var pricesResponse AzureComputePricesResponse
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&pricesResponse); err != nil {
			return nil, fmt.Errorf("failed to decode JSON response: %v", err)
		}

		for _, item := range pricesResponse.Items {
			allItems = append(allItems, ultron.ComputeCost{
				Currency:     &item.CurrencyCode,
				Unit:         &item.UnitOfMeasure,
				PricePerUnit: func(f float64) *float64 { v := float64(f); return &v }(item.UnitPrice),
			})
		}

		if pricesResponse.NextPageLink == "" {
			break
		}

		u, err = url.Parse(pricesResponse.NextPageLink)
		if err != nil {
			return nil, fmt.Errorf("failed to parse NextPageLink: %v", err)
		}
	}

	return &allItems, nil
}
