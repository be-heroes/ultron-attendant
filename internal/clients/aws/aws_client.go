package aws

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	pricingTypes "github.com/aws/aws-sdk-go-v2/service/pricing/types"
	ultron "github.com/be-heroes/ultron/pkg"
)

type IAwsClient interface {
	GetComputeCost(ctx context.Context, instanceType, region string) (*[]ultron.ComputeCost, error)
}

type IPricingAPI interface {
	GetProducts(ctx context.Context, params *pricing.GetProductsInput, optFns ...func(*pricing.Options)) (*pricing.GetProductsOutput, error)
}

// TODO: Refactor client to return compute configs, as well as compute costs and adhere to the same interface as the emma & wisp clients
type AwsClient struct {
	config        aws.Config
	PricingClient IPricingAPI
}

func NewAwsClient(region string) (*AwsClient, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(region))
	if err != nil {
		return nil, err
	}

	pricingCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion("us-east-1"))
	if err != nil {
		return nil, err
	}

	pricingClient := pricing.NewFromConfig(pricingCfg)

	return &AwsClient{
		config:        awsCfg,
		PricingClient: pricingClient,
	}, nil
}

func (c *AwsClient) GetComputeCost(ctx context.Context, instanceType, region string) (*[]ultron.ComputeCost, error) {
	input := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters: []pricingTypes.Filter{
			{
				Type:  pricingTypes.FilterTypeTermMatch,
				Field: aws.String("instanceType"),
				Value: aws.String(instanceType),
			},
			{
				Type:  pricingTypes.FilterTypeTermMatch,
				Field: aws.String("location"),
				Value: aws.String(region),
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

	var results []ultron.ComputeCost

	paginator := pricing.NewGetProductsPaginator(c.PricingClient, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, priceItem := range output.PriceList {
			var priceMap map[string]interface{}
			if err := json.Unmarshal([]byte(priceItem), &priceMap); err != nil {
				return nil, err
			}

			terms, ok := priceMap["terms"].(map[string]interface{})
			if !ok {
				continue
			}

			onDemand, ok := terms["OnDemand"].(map[string]interface{})
			if !ok {
				continue
			}

			for _, term := range onDemand {
				termAttributes, ok := term.(map[string]interface{})
				if !ok {
					continue
				}

				priceDimensions, ok := termAttributes["priceDimensions"].(map[string]interface{})
				if !ok {
					continue
				}

				for _, priceDimension := range priceDimensions {
					priceAttrs, ok := priceDimension.(map[string]interface{})
					if !ok {
						continue
					}
					pricePerUnit, ok := priceAttrs["pricePerUnit"].(map[string]interface{})
					if !ok {
						continue
					}
					priceUSD, _ := pricePerUnit["USD"].(string)
					priceUSDValue, err := strconv.ParseFloat(priceUSD, 32)
					if err != nil {
						return nil, err
					}
					priceCurrency := "USD"
					priceUnit := "MONTHLY"

					results = append(results, ultron.ComputeCost{
						Unit:         &priceUnit,
						Currency:     &priceCurrency,
						PricePerUnit: &priceUSDValue,
					})
				}
			}
		}
	}

	return &results, nil
}
