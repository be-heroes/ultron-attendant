package azure_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	wrapper "github.com/be-heroes/ultron-attendant/internal/clients/azure"
	ultron "github.com/be-heroes/ultron/pkg"
	"github.com/stretchr/testify/assert"
)

func TestGetComputeCost(t *testing.T) {
	sampleResponse := wrapper.AzureComputePricesResponse{
		Items: []wrapper.AzureComputePrice{
			{
				CurrencyCode:  "USD",
				UnitOfMeasure: "1 Hour",
				UnitPrice:     0.096,
				ArmRegionName: "eastus",
				ProductName:   "Virtual Machines",
				SkuName:       "D2 v3",
				ServiceName:   "Virtual Machines",
				ServiceFamily: "Compute",
				ArmSkuName:    "Standard_D2_v3",
			},
		},
		Count:        1,
		NextPageLink: "",
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		filter := q.Get("$filter")
		expectedFilter := "serviceName eq 'Virtual Machines' and currencyCode eq 'USD'"
		assert.Equal(t, expectedFilter, filter, "Expected filter does not match")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sampleResponse)
	}))
	defer testServer.Close()

	httpClient := &http.Client{}

	client := wrapper.NewAzureClient(httpClient, testServer.URL)

	ctx := context.Background()
	filter := "serviceName eq 'Virtual Machines' and currencyCode eq 'USD'"

	computeCosts, err := client.GetComputeCost(ctx, filter)
	assert.NoError(t, err, "Expected no error from GetComputeCost")

	assert.NotNil(t, computeCosts, "Expected computeCosts not to be nil")
	assert.Equal(t, len(sampleResponse.Items), len(*computeCosts), "Expected number of compute costs does not match")

	for i, cost := range *computeCosts {
		expectedItem := sampleResponse.Items[i]
		assert.NotNil(t, cost.Currency, "Expected Currency not to be nil")
		assert.Equal(t, expectedItem.CurrencyCode, *cost.Currency, "Currency does not match")

		assert.NotNil(t, cost.Unit, "Expected Unit not to be nil")
		assert.Equal(t, expectedItem.UnitOfMeasure, *cost.Unit, "Unit does not match")

		assert.NotNil(t, cost.PricePerUnit, "Expected PricePerUnit not to be nil")
		assert.Equal(t, float32(expectedItem.UnitPrice), *cost.PricePerUnit, "PricePerUnit does not match")
	}
}

func TestGetComputeCostWithPagination(t *testing.T) {
	firstPageResponse := wrapper.AzureComputePricesResponse{
		Items: []wrapper.AzureComputePrice{
			{
				CurrencyCode:  "USD",
				UnitOfMeasure: "1 Hour",
				UnitPrice:     0.096,
				ArmRegionName: "eastus",
				ProductName:   "Virtual Machines",
				SkuName:       "D2 v3",
				ServiceName:   "Virtual Machines",
				ServiceFamily: "Compute",
				ArmSkuName:    "Standard_D2_v3",
			},
		},
		Count:        1,
		NextPageLink: "/nextpage",
	}

	secondPageResponse := wrapper.AzureComputePricesResponse{
		Items: []wrapper.AzureComputePrice{
			{
				CurrencyCode:  "USD",
				UnitOfMeasure: "1 Hour",
				UnitPrice:     0.192,
				ArmRegionName: "eastus",
				ProductName:   "Virtual Machines",
				SkuName:       "D4 v3",
				ServiceName:   "Virtual Machines",
				ServiceFamily: "Compute",
				ArmSkuName:    "Standard_D4_v3",
			},
		},
		Count:        1,
		NextPageLink: "",
	}

	requestCount := 0

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 0 {
			json.NewEncoder(w).Encode(firstPageResponse)
		} else if requestCount == 1 {
			json.NewEncoder(w).Encode(secondPageResponse)
		} else {
			t.Fatalf("Unexpected number of requests: %d", requestCount)
		}
		requestCount++
	}))
	defer testServer.Close()

	firstPageResponse.NextPageLink = testServer.URL + "/nextpage"

	httpClient := &http.Client{}

	client := wrapper.NewAzureClient(httpClient, testServer.URL)

	ctx := context.Background()
	filter := "serviceName eq 'Virtual Machines' and currencyCode eq 'USD'"

	computeCosts, err := client.GetComputeCost(ctx, filter)
	assert.NoError(t, err, "Expected no error from GetComputeCost")

	expectedItems := []ultron.ComputeCost{
		{
			Currency:     &firstPageResponse.Items[0].CurrencyCode,
			Unit:         &firstPageResponse.Items[0].UnitOfMeasure,
			PricePerUnit: func(f float64) *float32 { v := float32(f); return &v }(firstPageResponse.Items[0].UnitPrice),
		},
		{
			Currency:     &secondPageResponse.Items[0].CurrencyCode,
			Unit:         &secondPageResponse.Items[0].UnitOfMeasure,
			PricePerUnit: func(f float64) *float32 { v := float32(f); return &v }(secondPageResponse.Items[0].UnitPrice),
		},
	}

	assert.NotNil(t, computeCosts, "Expected computeCosts not to be nil")
	assert.Equal(t, len(expectedItems), len(*computeCosts), "Expected number of compute costs does not match")

	for i, cost := range *computeCosts {
		expectedCost := expectedItems[i]
		assert.NotNil(t, cost.Currency, "Expected Currency not to be nil")
		assert.Equal(t, *expectedCost.Currency, *cost.Currency, "Currency does not match")

		assert.NotNil(t, cost.Unit, "Expected Unit not to be nil")
		assert.Equal(t, *expectedCost.Unit, *cost.Unit, "Unit does not match")

		assert.NotNil(t, cost.PricePerUnit, "Expected PricePerUnit not to be nil")
		assert.Equal(t, *expectedCost.PricePerUnit, *cost.PricePerUnit, "PricePerUnit does not match")
	}
}

func TestGetComputeCostEmptyResponse(t *testing.T) {
	sampleResponse := wrapper.AzureComputePricesResponse{
		Items:        []wrapper.AzureComputePrice{},
		Count:        0,
		NextPageLink: "",
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sampleResponse)
	}))
	defer testServer.Close()

	httpClient := &http.Client{}

	client := wrapper.NewAzureClient(httpClient, testServer.URL)

	ctx := context.Background()
	filter := "serviceName eq 'NonExistentService'"

	computeCosts, err := client.GetComputeCost(ctx, filter)
	assert.NoError(t, err, "Expected no error from GetComputeCost")
	assert.NotNil(t, computeCosts, "Expected computeCosts not to be nil")
	assert.Equal(t, 0, len(*computeCosts), "Expected zero compute costs")
}

func TestGetComputeCostHTTPError(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer testServer.Close()

	httpClient := &http.Client{}

	client := wrapper.NewAzureClient(httpClient, testServer.URL)

	ctx := context.Background()
	filter := "serviceName eq 'Virtual Machines'"

	_, err := client.GetComputeCost(ctx, filter)
	assert.Error(t, err, "Expected an error from GetComputeCost")
	assert.Contains(t, err.Error(), "non-OK HTTP status: 500 Internal Server Error", "Error message does not match")
}

func TestGetComputeCostInvalidJSON(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{ invalid json `))
	}))
	defer testServer.Close()

	httpClient := &http.Client{}

	client := wrapper.NewAzureClient(httpClient, testServer.URL)

	ctx := context.Background()
	filter := "serviceName eq 'Virtual Machines'"

	_, err := client.GetComputeCost(ctx, filter)
	assert.Error(t, err, "Expected an error from GetComputeCost")
	assert.Contains(t, err.Error(), "failed to decode JSON response", "Error message does not contain expected text")
}
