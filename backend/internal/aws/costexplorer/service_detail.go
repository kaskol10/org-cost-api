package costexplorer

import (
	"context"
	"fmt"
	"sort"
	"strings"

	ce "github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

type ServiceDetail struct {
	AccountID   string  `json:"account_id"`
	Service     string  `json:"service"`
	Start       string  `json:"start"`
	End         string  `json:"end"`
	Total       float64 `json:"total"`
	Unit        string  `json:"unit"`
	Daily       []DailyCost `json:"daily"`
	ByRegion    []ServiceGroupCost `json:"by_region"`
	ByUsageType []ServiceGroupCost `json:"by_usage_type"`
	ByOperation []ServiceGroupCost `json:"by_operation"`

	// Costs broken down by cost allocation tag key "Name" (when available).
	ByNameTag []ServiceGroupCost `json:"by_name_tag,omitempty"`
}

type ServiceGroupCost struct {
	Key     string  `json:"key"`
	Amount  float64 `json:"amount"`
	Unit    string  `json:"unit"`
	Percent float64 `json:"percent,omitempty"`
}

func GetServiceDetail(ctx context.Context, client *ce.Client, accountID, service, start, end string, fromPayer bool) (*ServiceDetail, error) {
	if strings.TrimSpace(service) == "" {
		return nil, fmt.Errorf("service is required")
	}
	linked := linkedAccountID(accountID, fromPayer)
	serviceExpr := types.Expression{
		Dimensions: &types.DimensionValues{
			Key:    types.DimensionService,
			Values: []string{service},
		},
	}
	// Usage-only, for this account + service.
	// Do not put an empty LinkedAccount expression into And — CE rejects it with
	// "Unknown expression is not allowed".
	var base types.Expression
	if linked != "" {
		base = types.Expression{
			And: []types.Expression{
				allServicesFilter(linked),
				serviceExpr,
			},
		}
	} else {
		base = serviceExpr
	}
	filter := usageOnlyFilter(base)

	daily, unit, err := getDailyCosts(ctx, client, start, end, filter)
	if err != nil {
		return nil, err
	}
	var total float64
	for _, d := range daily {
		total += d.Amount
	}

	byRegionMap, err := getGroupedCosts(ctx, client, start, end, filter, types.GroupDefinitionTypeDimension, string(types.DimensionRegion))
	if err != nil {
		return nil, err
	}
	byUsageTypeMap, err := getGroupedCosts(ctx, client, start, end, filter, types.GroupDefinitionTypeDimension, string(types.DimensionUsageType))
	if err != nil {
		return nil, err
	}
	byOperationMap, err := getGroupedCosts(ctx, client, start, end, filter, types.GroupDefinitionTypeDimension, string(types.DimensionOperation))
	if err != nil {
		// Not all services populate operation; treat as empty.
		byOperationMap = map[string]float64{}
	}

	var byNameTag []ServiceGroupCost
	// When grouping by TAG, Key must be the activated cost allocation tag key.
	// This works across services (S3, RDS, Kinesis, ...) when their costs are tag-attributable.
	tagMap, tagErr := getGroupedCosts(ctx, client, start, end, filter, types.GroupDefinitionTypeTag, "Name")
	if tagErr == nil {
		byNameTag = toTagGroupCosts("Name", tagMap, total, unit, 40)
	}

	return &ServiceDetail{
		AccountID:   accountID,
		Service:     service,
		Start:       start,
		End:         end,
		Total:       total,
		Unit:        unit,
		Daily:       daily,
		ByRegion:    toGroupCosts(byRegionMap, total, unit, 25),
		ByUsageType: toGroupCosts(byUsageTypeMap, total, unit, 40),
		ByOperation: toGroupCosts(byOperationMap, total, unit, 25),
		ByNameTag:  byNameTag,
	}, nil
}

func toGroupCosts(m map[string]float64, total float64, unit string, limit int) []ServiceGroupCost {
	out := make([]ServiceGroupCost, 0, len(m))
	for k, v := range m {
		if v <= 0 {
			continue
		}
		key := strings.TrimSpace(k)
		if key == "" {
			key = "Unknown"
		}
		pct := 0.0
		if total > 0 {
			pct = (v / total) * 100
		}
		out = append(out, ServiceGroupCost{Key: key, Amount: v, Unit: unit, Percent: pct})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Amount > out[j].Amount })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func toTagGroupCosts(tagKey string, m map[string]float64, total float64, unit string, limit int) []ServiceGroupCost {
	normalized := make(map[string]float64, len(m))
	prefix := tagKey + "$"
	for k, v := range m {
		key := strings.TrimSpace(k)
		key = strings.TrimPrefix(key, prefix)
		if key == "" {
			key = "Untagged"
		}
		normalized[key] += v
	}
	return toGroupCosts(normalized, total, unit, limit)
}

