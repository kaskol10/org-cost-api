package service

import (
	"context"
	"fmt"

	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
	awsclient "github.com/kaskol10/org-cost-api/backend/internal/aws"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
	ce "github.com/aws/aws-sdk-go-v2/service/costexplorer"
)

// ServiceTagTotals groups a Cost Explorer SERVICE by a cost allocation tag key (default: Name)
// and returns the most expensive buckets by current spend (no deltas).
func (a *Aggregator) ServiceTagTotals(
	ctx context.Context,
	accountID string,
	service string,
	start string,
	end string,
	tagKey string,
	topBuckets int,
) (*analysis.ServiceTagTotalsResponse, error) {
	if service == "" {
		return nil, fmt.Errorf("service is required")
	}
	if tagKey == "" {
		tagKey = "Name"
	}
	if topBuckets <= 0 {
		topBuckets = 10
	}
	if start == "" || end == "" {
		start, end = a.cfg.CostDateRange()
	}
	if start == "" || end == "" {
		return nil, fmt.Errorf("invalid date range")
	}

	var client *ce.Client
	var fromPayer bool
	if accountID != "" {
		var acctClient *awsclient.AccountClients
		for _, c := range a.clients {
			if c.AccountID == accountID {
				acctClient = c
				break
			}
		}
		if acctClient == nil {
			return nil, NewUnknownAccountError(accountID)
		}
		client, fromPayer = a.resolveCostClient(acctClient)
	} else {
		if a.billingCost == nil {
			return nil, fmt.Errorf("billing profile client not configured")
		}
		client = a.billingCost
		fromPayer = false
	}
	if client == nil {
		return nil, fmt.Errorf("no cost explorer client available")
	}

	tagMap, err := costexplorer.GetServiceTagTotals(ctx, client, accountID, service, start, end, fromPayer, tagKey)
	if err != nil {
		return nil, err
	}

	normalized := costexplorer.NormalizeTagMapKeys(tagKey, tagMap)
	var total float64
	for _, v := range normalized {
		total += v
	}

	buckets := analysis.TopBucketsByAmount(normalized, total, topBuckets)

	return &analysis.ServiceTagTotalsResponse{
		Service:        service,
		DisplayName:    analysis.FriendlyService(service),
		TagKey:         tagKey,
		CurrentPeriod:  analysis.PeriodSummary{Start: start, End: end},
		TotalCurrentUSD: total,
		Buckets:        buckets,
		CECallsUsed:    1,
	}, nil
}

