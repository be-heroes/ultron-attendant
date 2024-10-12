package aws_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	pricingTypes "github.com/aws/aws-sdk-go-v2/service/pricing/types"
	wrapper "github.com/be-heroes/ultron-attendant/internal/clients/aws"
	mocks "github.com/be-heroes/ultron-attendant/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetNodePriceData_Success(t *testing.T) {
	mockPricingClient := new(mocks.IPricingAPI)

	priceItem := map[string]interface{}{
		"product": map[string]interface{}{
			"attributes": map[string]interface{}{
				"instanceType": "t2.micro",
				"location":     "US East (N. Virginia)",
			},
		},
		"terms": map[string]interface{}{
			"OnDemand": map[string]interface{}{
				"XYZ": map[string]interface{}{
					"priceDimensions": map[string]interface{}{
						"ABC": map[string]interface{}{
							"pricePerUnit": map[string]interface{}{
								"USD": "0.0116",
							},
						},
					},
				},
			},
		},
	}

	priceItemJSON, _ := json.Marshal(priceItem)
	priceList := []string{string(priceItemJSON)}

	expectedInput := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters: []pricingTypes.Filter{
			{
				Type:  pricingTypes.FilterTypeTermMatch,
				Field: aws.String("instanceType"),
				Value: aws.String("t2.micro"),
			},
			{
				Type:  pricingTypes.FilterTypeTermMatch,
				Field: aws.String("location"),
				Value: aws.String("US East (N. Virginia)"),
			},
			{
				Type:  pricingTypes.FilterTypeTermMatch,
				Field: aws.String("operatingSystem"),
				Value: aws.String("Linux"),
			},
			{
				Type:  pricingTypes.FilterTypeTermMatch,
				Field: aws.String("preInstalledSw"),
				Value: aws.String("NA"),
			},
			{
				Type:  pricingTypes.FilterTypeTermMatch,
				Field: aws.String("tenancy"),
				Value: aws.String("Shared"),
			},
		},
	}

	mockPricingClient.On("GetProducts", mock.Anything, expectedInput, mock.Anything).
		Return(&pricing.GetProductsOutput{
			PriceList: priceList,
		}, nil)

	client := &wrapper.AwsClient{
		PricingClient: mockPricingClient,
	}

	_, err := client.GetComputeCost(context.Background(), "t2.micro", "US East (N. Virginia)")
	assert.NoError(t, err)

	mockPricingClient.AssertExpectations(t)
}

func TestGetNodePriceData_Error(t *testing.T) {
	mockPricingClient := new(mocks.IPricingAPI)

	mockPricingClient.On("GetProducts", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("AWS Pricing API error"))

	client := &wrapper.AwsClient{
		PricingClient: mockPricingClient,
	}

	_, err := client.GetComputeCost(context.Background(), "t2.micro", "US East (N. Virginia)")
	assert.Error(t, err)
	assert.EqualError(t, err, "AWS Pricing API error")

	mockPricingClient.AssertExpectations(t)
}
