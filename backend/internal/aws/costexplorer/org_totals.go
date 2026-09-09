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

// AccountCost is org-wide spend for one linked account.
type AccountCost struct {
	AccountID string  `json:"account_id"`
	Amount    float64 `json:"amount"`
	Unit      string  `json:"unit"`
	Percent   float64 `json:"percent,omitempty"`
}

// GetOrgAccountTotals returns org-wide linked-account totals for a date range (one CE API call).
func GetOrgAccountTotals(ctx context.Context, client *costexplorer.Client, start, end string) ([]AccountCost, float64, error) {
	if client == nil {
		return nil, 0, fmt.Errorf("cost explorer client is nil")
	}
	filter := usageOnlyFilter(allServicesFilter(""))
	byAccount, err := getGroupedCosts(ctx, client, start, end, filter, types.GroupDefinitionTypeDimension, string(types.DimensionLinkedAccount))
	if err != nil {
		return nil, 0, err
	}
	var total float64
	for _, amt := range byAccount {
		total += amt
	}
	out := make([]AccountCost, 0, len(byAccount))
	for id, amt := range byAccount {
		if amt <= 0 {
			continue
		}
		ac := AccountCost{AccountID: id, Amount: amt, Unit: "USD"}
		if total > 0 {
			ac.Percent = (amt / total) * 100
		}
		out = append(out, ac)
	}
	return out, total, nil
}
