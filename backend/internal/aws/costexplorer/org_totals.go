package costexplorer

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

// GetOrgServiceTotals returns org-wide service totals for a date range (one CE API call).
func GetOrgServiceTotals(ctx context.Context, client *costexplorer.Client, start, end string) ([]ServiceCost, float64, error) {
	if client == nil {
		return nil, 0, fmt.Errorf("cost explorer client is nil")
	}
	filter := usageOnlyFilter(allServicesFilter(""))
	byService, err := getGroupedCosts(ctx, client, start, end, filter, types.GroupDefinitionTypeDimension, string(types.DimensionService))
	if err != nil {
		return nil, 0, err
	}
	var total float64
	for _, amt := range byService {
		total += amt
	}
	return toServiceCosts(byService, total, "USD", 0), total, nil
}
