package costexplorer

import (
	"context"
	"sort"

	ce "github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func enrichCategoriesWithOperations(
	ctx context.Context,
	ceClient *ce.Client,
	ec2Client *ec2.Client,
	accountID, linkedAccountID, accountName, region, start, end string,
	categories []UsageCategory,
	byUsage map[string]float64,
	_ float64,
	storage *StorageContext,
) []UsageCategory {
	for i := range categories {
		switch categories[i].ID {
		case "vpc-peering":
			enrichVPCPeeringCategory(ctx, ceClient, ec2Client, accountID, linkedAccountID, region, start, end, &categories[i], byUsage)
			continue
		case "ebs-volumes":
			enrichEBSVolumesCategory(ctx, ceClient, ec2Client, linkedAccountID, start, end, &categories[i], byUsage, storage)
			continue
		case "ebs-snapshots":
			enrichEBSSnapshotsCategory(ctx, ceClient, ec2Client, accountID, linkedAccountID, accountName, region, start, end, &categories[i], byUsage, storage)
			continue
		}
		if !categoryIDsWithOperationDetail[categories[i].ID] {
			continue
		}
		keys := usageTypeKeysForCategory(byUsage, categories[i].ID)
		if len(keys) == 0 {
			continue
		}
		filter := filterForUsageTypes(linkedAccountID, keys)
		ops, err := getGroupedCosts(ctx, ceClient, start, end, filter, types.GroupDefinitionTypeDimension, string(types.DimensionOperation))
		if err != nil {
			continue
		}
		categories[i].APIOperations = toAPIOpCosts(ops, categories[i].Amount)
	}
	return categories
}

func filterForUsageTypes(linkedAccountID string, usageTypes []string) types.Expression {
	var usageExprs []types.Expression
	for _, ut := range usageTypes {
		usageExprs = append(usageExprs, types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionUsageType,
				Values: []string{ut},
			},
		})
	}
	usageFilter := types.Expression{Or: usageExprs}
	return types.Expression{
		And: []types.Expression{
			ec2OtherFilter(linkedAccountID),
			usageFilter,
		},
	}
}

func toAPIOpCosts(m map[string]float64, categoryTotal float64) []APIOperationCost {
	out := make([]APIOperationCost, 0, len(m))
	for k, v := range m {
		if v <= 0 {
			continue
		}
		label := k
		if label == "" {
			label = "StorageAndRecurring"
		}
		pct := 0.0
		if categoryTotal > 0 {
			pct = (v / categoryTotal) * 100
		}
		out = append(out, APIOperationCost{
			Operation: formatOperationLabel(label),
			Amount:    v,
			Unit:      "USD",
			Percent:   pct,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Amount > out[j].Amount })
	return out
}

func formatOperationLabel(op string) string {
	switch op {
	case "":
		return "Storage & recurring (no API op)"
	case "StorageAndRecurring":
		return "Storage & recurring (no API op)"
	default:
		return op
	}
}
