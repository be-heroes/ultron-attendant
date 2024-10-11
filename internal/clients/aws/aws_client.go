package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	pricingTypes "github.com/aws/aws-sdk-go-v2/service/pricing/types"
)

type IAwsClient interface {
	GetComputeCost(ctx context.Context, instanceType, region string) error
}

type IPricingAPI interface {
	GetProducts(ctx context.Context, params *pricing.GetProductsInput, optFns ...func(*pricing.Options)) (*pricing.GetProductsOutput, error)
}

type AwsClient struct {
	config        aws.Config
	PricingClient IPricingAPI
}

func NewAwsClient(region string) (*AwsClient, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(region))
	if err != nil {
		return nil, err
	}

	//TODO: The Pricing service is available only in specific regions. Figure out if this is a problem.
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

func (c *AwsClient) GetComputeCost(ctx context.Context, instanceType, region string) error {
	input := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		// TODO: Validate the filters
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

	paginator := pricing.NewGetProductsPaginator(c.PricingClient, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, priceItem := range output.PriceList {
			var priceMap map[string]interface{}
			if err := json.Unmarshal([]byte(priceItem), &priceMap); err != nil {
				return err
			}

			// TODO: Validate attributes
			product, ok := priceMap["product"].(map[string]interface{})
			if !ok {
				continue
			}

			attributes, ok := product["attributes"].(map[string]interface{})
			if !ok {
				continue
			}

			instanceTypeAttr, _ := attributes["instanceType"].(string)
			location, _ := attributes["location"].(string)
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

					// TODO: Map this to ultron.ComputeCost struct
					fmt.Printf("Instance Type: %s, Location: %s, Price per Hour: %s USD\n", instanceTypeAttr, location, priceUSD)
				}
			}
		}
	}

	return nil
}
