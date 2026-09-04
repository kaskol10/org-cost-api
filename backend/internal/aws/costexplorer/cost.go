package costexplorer

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ce "github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// ec2OtherServiceNames are SERVICE dimension values in Cost Explorer (console label: EC2-Other).
// The API uses "EC2 - Other", not "Amazon Elastic Compute Cloud - Other".
var ec2OtherServiceNames = []string{
	"EC2 - Other",
	"Amazon Elastic Compute Cloud - Other", // legacy / rare
}

// excludedRecordTypes mirrors Cost Explorer "exclude Refunds/Credits" charge-type filtering.
// Filtering RECORD_TYPE=Usage alone often returns $0 via the API (see aws/aws-sdk#40).
var excludedRecordTypes = []string{"Credit", "Refund"}

type DailyCost struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}

type UsageTypeCost struct {
	UsageType string  `json:"usage_type"`
	Region    string  `json:"region,omitempty"`
	ShortName string  `json:"short_name,omitempty"`
	Amount    float64 `json:"amount"`
	Unit      string  `json:"unit"`
	Percent   float64 `json:"percent,omitempty"`
}

// UsageTypeDaily is daily spend for one usage type (top types only).
type UsageTypeDaily struct {
	UsageType string      `json:"usage_type"`
	Category  string      `json:"category"`
	Total     float64     `json:"total"`
	Daily     []DailyCost `json:"daily"`
}

type APIOperationCost struct {
	Operation string  `json:"operation"`
	Amount    float64 `json:"amount"`
	Unit      string  `json:"unit"`
	Percent   float64 `json:"percent,omitempty"`
}

type ServiceCost struct {
	Service string  `json:"service"`
	Amount  float64 `json:"amount"`
	Unit    string  `json:"unit"`
	Percent float64 `json:"percent,omitempty"`
}

type SnapshotUsageCost struct {
	UsageType string  `json:"usage_type"`
	Amount    float64 `json:"amount"`
	Unit      string  `json:"unit"`
}

type CostSummary struct {
	AccountID   string             `json:"account_id"`
	AccountName string             `json:"account_name"`
	Start       string             `json:"start"`
	End         string             `json:"end"`
	// Total is the EC2-Other total (service filter = EC2 - Other).
	Total       float64            `json:"total"`
	Unit        string             `json:"unit"`
	Daily       []DailyCost        `json:"daily"`
	ByUsageType      []UsageTypeCost    `json:"by_usage_type"`
	UsageCategories  []UsageCategory    `json:"usage_categories"`
	TopUsageDaily    []UsageTypeDaily   `json:"top_usage_daily"`
	ByAPIOp          []APIOperationCost `json:"by_api_operation"`
	Snapshots        []SnapshotUsageCost `json:"snapshot_usage"`

	// All-services (usage-only) view to surface non-EC2-Other costs.
	AllTotal           float64       `json:"all_total"`
	AllDaily           []DailyCost   `json:"all_daily"`
	ByService          []ServiceCost `json:"by_service"`
	OtherServicesTotal float64       `json:"other_services_total"`
}

const topUsageTypesForDaily = 8
const topServicesForSummary = 25

func ec2OtherFilter(linkedAccountID string) types.Expression {
	var serviceExprs []types.Expression
	for _, name := range ec2OtherServiceNames {
		serviceExprs = append(serviceExprs, types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionService,
				Values: []string{name},
			},
		})
	}
	filter := types.Expression{Or: serviceExprs}
	if linkedAccountID != "" {
		filter = types.Expression{
			And: []types.Expression{
				filter,
				{
					Dimensions: &types.DimensionValues{
						Key:    types.DimensionLinkedAccount,
						Values: []string{linkedAccountID},
					},
				},
			},
		}
	}
	return filter
}

func allServicesFilter(linkedAccountID string) types.Expression {
	if linkedAccountID == "" {
		return types.Expression{}
	}
	return types.Expression{
		Dimensions: &types.DimensionValues{
			Key:    types.DimensionLinkedAccount,
			Values: []string{linkedAccountID},
		},
	}
}

func usageOnlyFilter(base types.Expression) types.Expression {
	usageExpr := types.Expression{
		Not: &types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionRecordType,
				Values: excludedRecordTypes,
			},
		},
	}
	if isEmptyExpression(base) {
		return usageExpr
	}
	return types.Expression{And: []types.Expression{base, usageExpr}}
}

func linkedAccountID(accountID string, fromPayer bool) string {
	if fromPayer {
		return accountID
	}
	return ""
}

func isEmptyExpression(e types.Expression) bool {
	return e.Dimensions == nil &&
		e.Tags == nil &&
		e.CostCategories == nil &&
		e.Not == nil &&
		len(e.And) == 0 &&
		len(e.Or) == 0
}

func GetCostSummary(ctx context.Context, client *ce.Client, ec2Client *ec2.Client, accountID, accountName, region, start, end string, fromPayer bool, storage *StorageContext) (*CostSummary, error) {
	linked := linkedAccountID(accountID, fromPayer)
	filter := ec2OtherFilter(linked)

	// One call: daily totals + daily grouped by usage type.
	daily, unit, byUsage, byUsageDaily, err := getDailyAndGroupedCosts(ctx, client, start, end, filter, string(types.DimensionUsageType))
	if err != nil {
		return nil, err
	}

	byAPI, err := getGroupedCosts(ctx, client, start, end, filter, types.GroupDefinitionTypeDimension, string(types.DimensionOperation))
	if err != nil {
		// API operation dimension may be empty for some accounts; non-fatal.
		byAPI = nil
	}

	var total float64
	for _, d := range daily {
		total += d.Amount
	}

	byUsageList := toUsageTypeCosts(byUsage, total)
	topDaily := usageTypeDailyFromGrouped(byUsage, byUsageDaily, topUsageTypesForDaily)

	// All-services (usage-only) totals for surfacing blind costs.
	allFilter := usageOnlyFilter(allServicesFilter(linked))
	// One call: daily totals + daily grouped by service.
	allDaily, allUnit, byServiceMap, _, err := getDailyAndGroupedCosts(ctx, client, start, end, allFilter, string(types.DimensionService))
	if err != nil {
		return nil, err
	}
	var allTotal float64
	for _, d := range allDaily {
		allTotal += d.Amount
	}
	byService := toServiceCosts(byServiceMap, allTotal, allUnit, topServicesForSummary)

	summary := &CostSummary{
		AccountID:       accountID,
		AccountName:     accountName,
		Start:           start,
		End:             end,
		Total:           total,
		Unit:            unit,
		Daily:           daily,
		ByUsageType:     byUsageList,
		UsageCategories: enrichCategoriesWithOperations(ctx, client, ec2Client, accountID, linked, accountName, region, start, end, buildUsageCategories(byUsage, total), byUsage, total, storage),
		TopUsageDaily:   topDaily,
		ByAPIOp:         toAPIOpCosts(byAPI, total),
		Snapshots:       snapshotCostsFromUsage(byUsage),

		AllTotal:   allTotal,
		AllDaily:   allDaily,
		ByService:  byService,
		OtherServicesTotal: allTotal - total,
	}
	return summary, nil
}

func usageTypeDailyFromGrouped(byUsage map[string]float64, byUsageDaily map[string]map[string]float64, limit int) []UsageTypeDaily {
	var out []UsageTypeDaily
	for _, usageType := range sortedTopKeys(byUsage, limit) {
		dates := byUsageDaily[usageType]
		if len(dates) == 0 {
			continue
		}
		var daily []DailyCost
		var typeTotal float64
		for date, amt := range dates {
			daily = append(daily, DailyCost{Date: date, Amount: amt, Unit: "USD"})
			typeTotal += amt
		}
		sort.Slice(daily, func(i, j int) bool { return daily[i].Date < daily[j].Date })
		out = append(out, UsageTypeDaily{
			UsageType: usageType,
			Category:  categorizeUsageType(usageType),
			Total:     typeTotal,
			Daily:     daily,
		})
	}
	return out
}

func getDailyAndGroupedCosts(
	ctx context.Context,
	client *ce.Client,
	start, end string,
	filter types.Expression,
	groupKey string,
) (daily []DailyCost, unit string, groupedTotals map[string]float64, groupedDaily map[string]map[string]float64, err error) {
	if err := withCESem(ctx); err != nil {
		return nil, "", nil, nil, err
	}
	defer releaseCESem()

	input := &ce.GetCostAndUsageInput{
		TimePeriod:  &types.DateInterval{Start: aws.String(start), End: aws.String(end)},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
		Filter:      &filter,
		GroupBy: []types.GroupDefinition{{
			Type: types.GroupDefinitionTypeDimension,
			Key:  aws.String(groupKey),
		}},
	}

	groupedTotals = make(map[string]float64)
	groupedDaily = make(map[string]map[string]float64)
	var nextToken *string
	for {
		input.NextPageToken = nextToken
		out, e := client.GetCostAndUsage(ctx, input)
		if e != nil {
			return nil, "", nil, nil, fmt.Errorf("get daily grouped cost (%s): %w", groupKey, e)
		}
		for _, result := range out.ResultsByTime {
			date := aws.ToString(result.TimePeriod.Start)
			amt, u := parseAmount(result.Total)
			if amt == 0 && len(result.Groups) > 0 {
				for _, group := range result.Groups {
					gAmt, _ := parseAmount(group.Metrics)
					amt += gAmt
				}
			}
			if unit == "" {
				unit = u
			}
			daily = append(daily, DailyCost{Date: date, Amount: amt, Unit: u})
			for _, group := range result.Groups {
				if len(group.Keys) == 0 {
					continue
				}
				key := strings.TrimSpace(group.Keys[0])
				if key == "" {
					continue
				}
				gAmt, _ := parseAmount(group.Metrics)
				groupedTotals[key] += gAmt
				if groupedDaily[key] == nil {
					groupedDaily[key] = make(map[string]float64)
				}
				groupedDaily[key][date] += gAmt
			}
		}
		if out.NextPageToken == nil {
			break
		}
		nextToken = out.NextPageToken
	}
	sort.Slice(daily, func(i, j int) bool { return daily[i].Date < daily[j].Date })
	return daily, unit, groupedTotals, groupedDaily, nil
}

// snapshotCostsFromUsage picks EBS snapshot lines from the usage-type breakdown.
// Cost Explorer filters only support EQUALS on USAGE_TYPE, not CONTAINS.
func snapshotCostsFromUsage(byUsage map[string]float64) []SnapshotUsageCost {
	var snaps []SnapshotUsageCost
	for usageType, amount := range byUsage {
		if strings.Contains(strings.ToLower(usageType), "snapshot") {
			snaps = append(snaps, SnapshotUsageCost{UsageType: displayUsageType(usageType), Amount: amount, Unit: "USD"})
		}
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].Amount > snaps[j].Amount })
	return snaps
}

func getDailyCosts(ctx context.Context, client *ce.Client, start, end string, filter types.Expression) ([]DailyCost, string, error) {
	if err := withCESem(ctx); err != nil {
		return nil, "", err
	}
	defer releaseCESem()

	out, err := client.GetCostAndUsage(ctx, &ce.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{Start: aws.String(start), End: aws.String(end)},
		Granularity: types.GranularityDaily,
		Metrics:    []string{"UnblendedCost"},
		Filter:     &filter,
	})
	if err != nil {
		return nil, "", fmt.Errorf("get daily cost: %w", err)
	}

	var unit string
	var daily []DailyCost
	for _, result := range out.ResultsByTime {
		date := aws.ToString(result.TimePeriod.Start)
		amt, u := parseAmount(result.Total)
		if unit == "" {
			unit = u
		}
		daily = append(daily, DailyCost{Date: date, Amount: amt, Unit: u})
	}
	sort.Slice(daily, func(i, j int) bool { return daily[i].Date < daily[j].Date })
	return daily, unit, nil
}

func getGroupedCosts(ctx context.Context, client *ce.Client, start, end string, filter types.Expression, groupType types.GroupDefinitionType, key string) (map[string]float64, error) {
	if err := withCESem(ctx); err != nil {
		return nil, err
	}
	defer releaseCESem()

	input := &ce.GetCostAndUsageInput{
		TimePeriod:  &types.DateInterval{Start: aws.String(start), End: aws.String(end)},
		Granularity: types.GranularityMonthly,
		Metrics:     []string{"UnblendedCost"},
		Filter:      &filter,
		GroupBy: []types.GroupDefinition{{
			Type: groupType,
			Key:  aws.String(key),
		}},
	}
	return paginatedGroupTotals(ctx, client, input)
}

func getDailyCostsForTopUsageTypes(ctx context.Context, client *ce.Client, start, end string, filter types.Expression, byUsage map[string]float64, limit int) ([]UsageTypeDaily, error) {
	top := topUsageTypeKeys(byUsage, limit)
	if len(top) == 0 {
		return nil, nil
	}

	input := &ce.GetCostAndUsageInput{
		TimePeriod:  &types.DateInterval{Start: aws.String(start), End: aws.String(end)},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
		Filter:      &filter,
		GroupBy: []types.GroupDefinition{{
			Type: types.GroupDefinitionTypeDimension,
			Key:  aws.String(string(types.DimensionUsageType)),
		}},
	}

	byTypeDate := make(map[string]map[string]float64)
	var nextToken *string
	for {
		input.NextPageToken = nextToken
		out, err := client.GetCostAndUsage(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("get daily usage type costs: %w", err)
		}
		for _, result := range out.ResultsByTime {
			date := aws.ToString(result.TimePeriod.Start)
			for _, group := range result.Groups {
				name := strings.TrimSpace(group.Keys[0])
				if name == "" || !top[name] {
					continue
				}
				amt, _ := parseAmount(group.Metrics)
				if byTypeDate[name] == nil {
					byTypeDate[name] = make(map[string]float64)
				}
				byTypeDate[name][date] += amt
			}
		}
		if out.NextPageToken == nil {
			break
		}
		nextToken = out.NextPageToken
	}

	var out []UsageTypeDaily
	for _, usageType := range sortedTopKeys(byUsage, limit) {
		dates := byTypeDate[usageType]
		if len(dates) == 0 {
			continue
		}
		var daily []DailyCost
		var typeTotal float64
		for date, amt := range dates {
			daily = append(daily, DailyCost{Date: date, Amount: amt, Unit: "USD"})
			typeTotal += amt
		}
		sort.Slice(daily, func(i, j int) bool { return daily[i].Date < daily[j].Date })
		out = append(out, UsageTypeDaily{
			UsageType: displayUsageType(usageType),
			Category:  categorizeUsageType(usageType),
			Total:     typeTotal,
			Daily:     daily,
		})
	}
	return out, nil
}

func topUsageTypeKeys(byUsage map[string]float64, limit int) map[string]bool {
	keys := sortedTopKeys(byUsage, limit)
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	return set
}

func sortedTopKeys(byUsage map[string]float64, limit int) []string {
	type kv struct {
		k string
		v float64
	}
	var list []kv
	for k, v := range byUsage {
		if v > 0 {
			list = append(list, kv{k, v})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v })
	if len(list) > limit {
		list = list[:limit]
	}
	keys := make([]string, len(list))
	for i, item := range list {
		keys[i] = item.k
	}
	return keys
}

func paginatedGroupTotals(ctx context.Context, client *ce.Client, input *ce.GetCostAndUsageInput) (map[string]float64, error) {
	totals := make(map[string]float64)
	var nextToken *string
	for {
		input.NextPageToken = nextToken
		out, err := client.GetCostAndUsage(ctx, input)
		if err != nil {
			key := ""
			if len(input.GroupBy) > 0 && input.GroupBy[0].Key != nil {
				key = *input.GroupBy[0].Key
			}
			return nil, fmt.Errorf("get grouped cost (%s): %w", key, err)
		}
		allowEmptyKey := groupKeyAllowsEmpty(input)
		for _, result := range out.ResultsByTime {
			for _, group := range result.Groups {
				if len(group.Keys) == 0 {
					continue
				}
				name := strings.TrimSpace(group.Keys[0])
				if name == "" && !allowEmptyKey {
					continue
				}
				amt, _ := parseAmount(group.Metrics)
				totals[name] += amt
			}
		}
		if out.NextPageToken == nil {
			break
		}
		nextToken = out.NextPageToken
	}
	return totals, nil
}

func parseAmount(metrics map[string]types.MetricValue) (float64, string) {
	m, ok := metrics["UnblendedCost"]
	if !ok {
		return 0, "USD"
	}
	amount, _ := strconv.ParseFloat(aws.ToString(m.Amount), 64)
	unit := aws.ToString(m.Unit)
	if unit == "" {
		unit = "USD"
	}
	return amount, unit
}

func toUsageTypeCosts(m map[string]float64, total float64) []UsageTypeCost {
	out := make([]UsageTypeCost, 0, len(m))
	for k, v := range m {
		if v <= 0 {
			continue
		}
		pct := 0.0
		if total > 0 {
			pct = (v / total) * 100
		}
		region, short := splitUsageTypeRegion(k)
		out = append(out, UsageTypeCost{
			UsageType: k,
			Region:    region,
			ShortName: short,
			Amount:    v,
			Unit:      "USD",
			Percent:   pct,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Amount > out[j].Amount })
	return out
}

func toServiceCosts(m map[string]float64, allTotal float64, unit string, limit int) []ServiceCost {
	out := make([]ServiceCost, 0, len(m))
	for svc, v := range m {
		if v <= 0 {
			continue
		}
		pct := 0.0
		if allTotal > 0 {
			pct = (v / allTotal) * 100
		}
		label := strings.TrimSpace(svc)
		if label == "" {
			label = "Unknown"
		}
		out = append(out, ServiceCost{
			Service: label,
			Amount:  v,
			Unit:    unit,
			Percent: pct,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Amount > out[j].Amount })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func groupKeyAllowsEmpty(input *ce.GetCostAndUsageInput) bool {
	if len(input.GroupBy) == 0 || input.GroupBy[0].Key == nil {
		return false
	}
	return *input.GroupBy[0].Key == string(types.DimensionOperation)
}

// FormatDateRange returns inclusive-style labels for the UI.
func FormatDateRange(start, end string) string {
	s, _ := time.Parse("2006-01-02", start)
	e, _ := time.Parse("2006-01-02", end)
	return fmt.Sprintf("%s → %s", s.Format("Jan 2, 2006"), e.Format("Jan 2, 2006"))
}
