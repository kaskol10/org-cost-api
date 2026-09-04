package costexplorer

import (
	"context"
	"fmt"
	"strings"

	ce "github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

// GetServiceTagTotals returns total unblended costs grouped by a cost allocation tag for one service.
//
// The tag keys returned by Cost Explorer are in the form: "<tagKey>$<tagValue>".
// Use NormalizeTagMapKeys to convert those to a normalized map keyed by "<tagValue>".
func GetServiceTagTotals(
	ctx context.Context,
	client *ce.Client,
	accountID string,
	service string,
	start string,
	end string,
	fromPayer bool,
	tagKey string,
) (map[string]float64, error) {
	if strings.TrimSpace(service) == "" {
		return nil, fmt.Errorf("service is required")
	}
	if strings.TrimSpace(tagKey) == "" {
		return nil, fmt.Errorf("tagKey is required")
	}

	linked := linkedAccountID(accountID, fromPayer)
	serviceExpr := types.Expression{
		Dimensions: &types.DimensionValues{
			Key:    types.DimensionService,
			Values: []string{service},
		},
	}
	var base types.Expression
	if linked != "" {
		// Only build an And when we actually have >= 2 operands.
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

	return getGroupedCosts(ctx, client, start, end, filter, types.GroupDefinitionTypeTag, tagKey)
}

// NormalizeTagMapKeys converts Cost Explorer tag group keys ("<tagKey>$<tagValue>") into a normalized map
// keyed by "<tagValue>".
//
// Empty tag values are normalized to "Untagged".
func NormalizeTagMapKeys(tagKey string, tagMap map[string]float64) map[string]float64 {
	prefix := tagKey + "$"
	out := make(map[string]float64, len(tagMap))
	for k, v := range tagMap {
		key := strings.TrimSpace(k)
		key = strings.TrimPrefix(key, prefix)
		key = strings.TrimSpace(key)
		if key == "" {
			key = "Untagged"
		}
		out[key] += v
	}
	return out
}

