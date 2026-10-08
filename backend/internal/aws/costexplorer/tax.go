package costexplorer

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

// TaxBreakdown isolates Tax records so they can be shown separately from
// usage. AWS posts the previous month's tax as a single lump on the 1st of the
// new month, which otherwise fakes a day-1 spike and skews daily series and
// period-over-period comparisons. Usage queries exclude Tax by default (see
// cost.go); this breakdown makes the excluded amount visible.
//
// TotalUSD is computed by the difference method (inclTaxTotal − usageTotal,
// both exclusion-filter queries) because positive RECORD_TYPE=Tax filtering is
// unreliable via the API (same reason usage filters by a negative RECORD_TYPE
// list — see cost.go aws/aws-sdk#40). ByService/Daily/PostedDays come from a
// best-effort positive RECORD_TYPE=Tax query and may be empty even when
// TotalUSD > 0; the UI falls back to the total-only view in that case.
type TaxBreakdown struct {
	// TotalUSD is the tax for the range, computed as inclTaxTotal − usageTotal.
	TotalUSD float64 `json:"total_usd"`
	// InclTaxTotalUSD is org usage+tax spend (excludes only Credits/Refunds).
	InclTaxTotalUSD float64 `json:"incl_tax_total_usd"`
	// ByService breaks the tax down by the SERVICE dimension (best-effort).
	ByService []ServiceCost `json:"by_service,omitempty"`
	// Daily is the per-day tax total (best-effort), revealing the lump-on-the-1st.
	Daily []DailyCost `json:"daily,omitempty"`
	// PostedDays is the number of distinct days tax was posted on (best-effort).
	PostedDays int `json:"posted_days,omitempty"`
	// HasData is true when non-zero tax was found.
	HasData bool `json:"has_data,omitempty"`
}

// Has reports whether the breakdown has any tax.
func (t *TaxBreakdown) Has() bool { return t != nil && t.TotalUSD > 0 }

// ReconcileTaxTotal computes tax as the difference between org spend with and
// without tax. Both inputs come from the same-scope exclusion-filter queries,
// so the result is reliable even when positive record-type filtering returns
// nothing. Floored at 0 to absorb minor CE rounding.
func ReconcileTaxTotal(inclTaxTotal, usageTotal float64) float64 {
	tax := inclTaxTotal - usageTotal
	if tax < 0 {
		tax = 0
	}
	return tax
}

// exclusionFilter builds a Not[RECORD_TYPE in values] expression.
func exclusionFilter(values []string) types.Expression {
	return types.Expression{
		Not: &types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionRecordType,
				Values: values,
			},
		},
	}
}

// orgTotalExcluding returns the org-wide unblended total for the range,
// excluding the given record types (one monthly CE call).
func orgTotalExcluding(ctx context.Context, client *costexplorer.Client, start, end string, excl []string) (float64, int, error) {
	if err := withCESem(ctx); err != nil {
		return 0, 0, err
	}
	defer releaseCESem()

	filter := exclusionFilter(excl)
	out, err := client.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod:  &types.DateInterval{Start: aws.String(start), End: aws.String(end)},
		Granularity: types.GranularityMonthly,
		Metrics:     []string{"UnblendedCost"},
		Filter:      &filter,
	})
	if err != nil {
		return 0, 1, fmt.Errorf("get org total (excl %v): %w", excl, err)
	}
	var total float64
	for _, result := range out.ResultsByTime {
		amt, _ := parseAmount(result.Total)
		total += amt
	}
	return total, 1, nil
}

// GetOrgTaxTotals runs the two same-scope exclusion-filter queries and
// reconciles the tax total. Returns the incl-tax total, the usage (tax-free)
// total, the reconciled tax, and the number of CE calls made.
func GetOrgTaxTotals(ctx context.Context, client *costexplorer.Client, start, end string) (inclTax, usage, tax float64, calls int, err error) {
	if client == nil {
		return 0, 0, 0, 0, fmt.Errorf("cost explorer client is nil")
	}
	inclTax, c1, err := orgTotalExcluding(ctx, client, start, end, []string{"Credit", "Refund"})
	calls += c1
	if err != nil {
		return 0, 0, 0, calls, err
	}
	usage, c2, err := orgTotalExcluding(ctx, client, start, end, []string{"Credit", "Refund", "Tax"})
	calls += c2
	if err != nil {
		return 0, 0, 0, calls, err
	}
	tax = ReconcileTaxTotal(inclTax, usage)
	return inclTax, usage, tax, calls, nil
}

// taxOnlyFilter matches Tax records across all services (best-effort detail).
func taxOnlyFilter() types.Expression {
	return types.Expression{
		Dimensions: &types.DimensionValues{
			Key:    types.DimensionRecordType,
			Values: []string{"Tax"},
		},
	}
}

// GetOrgTaxDetail runs a single best-effort RECORD_TYPE=Tax query (daily,
// grouped by SERVICE) to populate the per-service and per-day breakdown. It may
// return empty results even when tax exists (positive record-type filtering is
// unreliable); callers must treat ByService/Daily/PostedDays as supplementary
// to the difference-method TotalUSD.
func GetOrgTaxDetail(ctx context.Context, client *costexplorer.Client, start, end string) (byService []ServiceCost, daily []DailyCost, postedDays int, calls int, err error) {
	if client == nil {
		return nil, nil, 0, 0, fmt.Errorf("cost explorer client is nil")
	}
	if err := withCESem(ctx); err != nil {
		return nil, nil, 0, 0, err
	}
	defer releaseCESem()

	filter := taxOnlyFilter()
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod:  &types.DateInterval{Start: aws.String(start), End: aws.String(end)},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
		Filter:      &filter,
		GroupBy: []types.GroupDefinition{{
			Type: types.GroupDefinitionTypeDimension,
			Key:  aws.String(string(types.DimensionService)),
		}},
	}

	dailyByDate := make(map[string]float64)
	bySvcMap := make(map[string]float64)
	var nextToken *string
	var unit string
	for {
		input.NextPageToken = nextToken
		out, err := client.GetCostAndUsage(ctx, input)
		if err != nil {
			return nil, nil, 0, 1, fmt.Errorf("get tax detail: %w", err)
		}
		for _, result := range out.ResultsByTime {
			date := aws.ToString(result.TimePeriod.Start)
			dayTotal, u := parseAmount(result.Total)
			if u != "" {
				unit = u
			}
			for _, group := range result.Groups {
				if len(group.Keys) == 0 {
					continue
				}
				svc := strings.TrimSpace(group.Keys[0])
				gAmt, gu := parseAmount(group.Metrics)
				if gu != "" {
					unit = gu
				}
				if svc == "" {
					svc = "Tax"
				}
				bySvcMap[svc] += gAmt
			}
			// When grouped, result.Total is often 0; fall back to the group sum.
			if dayTotal == 0 {
				for _, group := range result.Groups {
					gAmt, _ := parseAmount(group.Metrics)
					dayTotal += gAmt
				}
			}
			if dayTotal != 0 {
				dailyByDate[date] += dayTotal
			}
		}
		if out.NextPageToken == nil {
			break
		}
		nextToken = out.NextPageToken
	}
	if unit == "" {
		unit = "USD"
	}

	var svcSum float64
	for _, v := range bySvcMap {
		svcSum += v
	}
	byService = make([]ServiceCost, 0, len(bySvcMap))
	for svc, v := range bySvcMap {
		if v <= 0 {
			continue
		}
		label := strings.TrimSpace(svc)
		if label == "" {
			label = "Tax"
		}
		pct := 0.0
		if svcSum > 0 {
			pct = v / svcSum * 100
		}
		byService = append(byService, ServiceCost{Service: label, Amount: v, Unit: unit, Percent: pct})
	}
	sort.Slice(byService, func(i, j int) bool { return byService[i].Amount > byService[j].Amount })

	daily = make([]DailyCost, 0, len(dailyByDate))
	for date, amt := range dailyByDate {
		daily = append(daily, DailyCost{Date: date, Amount: amt, Unit: unit})
	}
	sort.Slice(daily, func(i, j int) bool { return daily[i].Date < daily[j].Date })

	return byService, daily, len(daily), 1, nil
}
