package costexplorer

import (
	"sort"
	"strings"
)

// UsageCategory describes a rolled-up EC2-Other cost bucket.
type UsageCategory struct {
	ID           string             `json:"id"`
	Label        string             `json:"label"`
	Description  string             `json:"description"`
	Amount       float64            `json:"amount"`
	Percent      float64            `json:"percent"`
	UsageTypes         []UsageTypeCost      `json:"usage_types"`
	APIOperations      []APIOperationCost   `json:"api_operations,omitempty"`
	PeeringDetails     []PeeringConnectionDetail `json:"peering_details"`
	PeeringDriversNote string                    `json:"peering_drivers_note,omitempty"`
	EBSVolumeDetail    *EBSVolumeDetail          `json:"ebs_volume_detail,omitempty"`
	EBSSnapshotDetail  *EBSSnapshotDetail        `json:"ebs_snapshot_detail,omitempty"`
	InventoryCount     int                       `json:"inventory_count,omitempty"`
	InventorySizeGiB float64                   `json:"inventory_size_gib,omitempty"`
}

// categoryIDsWithOperationDetail are categories that get a Cost Explorer OPERATION breakdown.
var categoryIDsWithOperationDetail = map[string]bool{
	"nat-gateway": true,
}

var categoryDefs = []struct {
	ID          string
	Label       string
	Description string
	match       func(lower string) bool
}{
	{
		ID: "ebs-volumes", Label: "EBS volumes",
		Description: "Provisioned volume capacity (gp2, gp3, io1, etc.)",
		match: func(s string) bool {
			return strings.Contains(s, "volumeusage") || strings.Contains(s, "ebs:volume")
		},
	},
	{
		ID: "ebs-snapshots", Label: "EBS snapshots",
		Description: "Snapshot storage and snapshot API charges",
		match: func(s string) bool { return strings.Contains(s, "snapshot") },
	},
	{
		ID: "nat-gateway", Label: "NAT Gateway",
		Description: "NAT Gateway hourly and data processing charges",
		match: func(s string) bool { return strings.Contains(s, "natgateway") },
	},
	{
		ID: "vpc-peering", Label: "VPC peering",
		Description: "Cross-VPC peering data transfer",
		match: func(s string) bool { return strings.Contains(s, "vpcpeering") },
	},
	{
		ID: "data-transfer", Label: "Data transfer",
		Description: "Regional and cross-AZ/region data transfer",
		match: func(s string) bool {
			return strings.Contains(s, "datatransfer") ||
				strings.Contains(s, "aws-out-bytes") ||
				strings.Contains(s, "aws-in-bytes") ||
				strings.Contains(s, "regional-bytes")
		},
	},
	{
		ID: "elastic-ip", Label: "Elastic IP",
		Description: "Public IPv4 addresses (including idle addresses)",
		match: func(s string) bool {
			return strings.Contains(s, "elasticip") || strings.Contains(s, "idleaddress")
		},
	},
	{
		ID: "cpu-credits", Label: "CPU credits",
		Description: "T2/T3 unlimited mode CPU credit charges",
		match: func(s string) bool { return strings.Contains(s, "cpucredits") },
	},
}

func categorizeUsageType(usageType string) string {
	lower := strings.ToLower(usageType)
	for _, c := range categoryDefs {
		if c.match(lower) {
			return c.ID
		}
	}
	return "other"
}

func usageTypeKeysForCategory(byUsage map[string]float64, categoryID string) []string {
	var keys []string
	for usageType, amount := range byUsage {
		if amount > 0 && categorizeUsageType(usageType) == categoryID {
			keys = append(keys, usageType)
		}
	}
	return keys
}

func buildUsageCategories(byUsage map[string]float64, total float64) []UsageCategory {
	byCat := make(map[string][]UsageTypeCost)
	catTotals := make(map[string]float64)

	for usageType, amount := range byUsage {
		if amount <= 0 {
			continue
		}
		id := categorizeUsageType(usageType)
		region, short := splitUsageTypeRegion(usageType)
		byCat[id] = append(byCat[id], UsageTypeCost{
			UsageType: usageType,
			Region:    region,
			ShortName: short,
			Amount:    amount,
			Unit:      "USD",
		})
		catTotals[id] += amount
	}

	var categories []UsageCategory
	for _, def := range categoryDefs {
		items := byCat[def.ID]
		if len(items) == 0 {
			continue
		}
		sortUsageTypes(items)
		amt := catTotals[def.ID]
		pct := 0.0
		if total > 0 {
			pct = (amt / total) * 100
		}
		categories = append(categories, UsageCategory{
			ID:          def.ID,
			Label:       def.Label,
			Description: def.Description,
			Amount:      amt,
			Percent:     pct,
			UsageTypes:  items,
		})
	}

	if items := byCat["other"]; len(items) > 0 {
		sortUsageTypes(items)
		amt := catTotals["other"]
		pct := 0.0
		if total > 0 {
			pct = (amt / total) * 100
		}
		categories = append(categories, UsageCategory{
			ID:          "other",
			Label:       "Other",
			Description: "Remaining EC2-Other usage types",
			Amount:      amt,
			Percent:     pct,
			UsageTypes:  items,
		})
	}

	sort.Slice(categories, func(i, j int) bool { return categories[i].Amount > categories[j].Amount })
	return categories
}

// splitUsageTypeRegion splits EUW2-EBS:VolumeUsage.gp3 into EUW2 and EBS:VolumeUsage.gp3.
func splitUsageTypeRegion(usageType string) (region, short string) {
	if i := strings.Index(usageType, "-"); i > 0 && i <= 4 {
		prefix := usageType[:i]
		rest := usageType[i+1:]
		if len(rest) > 0 && rest[0] >= 'A' && rest[0] <= 'Z' {
			return prefix, rest
		}
	}
	return "", usageType
}

// displayUsageType strips the regional prefix for compact labels (charts).
func displayUsageType(usageType string) string {
	_, short := splitUsageTypeRegion(usageType)
	return short
}

func sortUsageTypes(items []UsageTypeCost) {
	sort.Slice(items, func(i, j int) bool { return items[i].Amount > items[j].Amount })
}
