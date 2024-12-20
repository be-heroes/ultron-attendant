package gcp

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/bigquery"
	attendant "github.com/be-heroes/ultron-attendant/pkg"
	ultron "github.com/be-heroes/ultron/pkg"
	"google.golang.org/api/cloudbilling/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type IGcpClient interface {
	GetComputeCost(ctx context.Context, projectId string) (*[]ultron.ComputeCost, error)
}

type GcpClient struct {
	credentials string
	billingSvc  *cloudbilling.APIService
	bqClient    *bigquery.Client
}

func NewGcpClient() (*GcpClient, error) {
	credentials := os.Getenv(attendant.EnvGoogleCredentials)
	if credentials == "" {
		return nil, fmt.Errorf("%s environment variable is not set", attendant.EnvGoogleCredentials)
	}

	ctx := context.Background()

	billingService, err := cloudbilling.NewService(ctx, option.WithCredentialsFile(credentials))
	if err != nil {
		return nil, fmt.Errorf("failed to create billing service: %v", err)
	}

	bqClient, err := bigquery.NewClient(ctx, "your-project-id", option.WithCredentialsFile(credentials))
	if err != nil {
		return nil, fmt.Errorf("failed to create BigQuery client: %v", err)
	}

	return &GcpClient{
		credentials: credentials,
		billingSvc:  billingService,
		bqClient:    bqClient,
	}, nil
}

func (g *GcpClient) GetComputeCost(ctx context.Context, projectId string) (*[]ultron.ComputeCost, error) {
	query := `
		SELECT 
			service.description AS service_name,
			sku.description AS sku_name,
			usage_start_time,
			usage_end_time,
			usage.amount AS usage_amount,
			usage.unit AS usage_unit,
			cost AS cost_in_usd,
			currency
		FROM 
			` + "`your-project-id.gcp_billing_dataset.gcp_billing_export`" + `
		WHERE 
			service.description = 'Compute Engine'
			AND project.id = @projectId
		ORDER BY
			usage_start_time DESC
	`

	q := g.bqClient.Query(query)
	q.Parameters = []bigquery.QueryParameter{
		{Name: "projectId", Value: projectId},
	}

	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to run BigQuery query: %v", err)
	}

	var computeCosts []ultron.ComputeCost

	for {
		var values []bigquery.Value
		err := it.Next(&values)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate through query results: %v", err)
		}

		// serviceName := values[0].(string)
		// skuName := values[1].(string)
		// usageStart := values[2].(string)
		// usageEnd := values[3].(string)
		// usageAmount := values[4].(float64)
		usageUnit := values[5].(string)
		cost := values[6].(float64)
		currency := values[7].(string)

		computeCost := ultron.ComputeCost{
			Unit:         &usageUnit,
			Currency:     &currency,
			PricePerUnit: func(f float64) *float64 { v := float64(f); return &v }(cost),
		}

		computeCosts = append(computeCosts, computeCost)
	}

	return &computeCosts, nil
}
