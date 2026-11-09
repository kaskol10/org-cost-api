package analysis

import (
	"math"
	"sort"
	"strings"
)

// EBSPricingBasis describes what the unattached-EBS estimate covers, so API
// consumers can show the caveat next to the number.
const EBSPricingBasis = "AWS list price per GiB-month (US East), storage only; excludes provisioned IOPS/throughput and regional differences"

// DefaultEBSPricePerGiBMonth prices volumes whose type is unknown (gp3 list price).
const DefaultEBSPricePerGiBMonth = 0.08

// ebsPricePerGiBMonth holds EBS storage list prices (USD per GiB-month).
// Source: https://aws.amazon.com/ebs/pricing/ ("standard" from the
// previous-generation volumes page).
var ebsPricePerGiBMonth = map[string]float64{
	"gp3":      0.08,
	"gp2":      0.10,
	"io1":      0.125,
	"io2":      0.125,
	"st1":      0.045,
	"sc1":      0.015,
	"standard": 0.05,
}

// EBSPricePerGiBMonth returns the storage list price for a volume type,
// falling back to DefaultEBSPricePerGiBMonth for unknown types.
func EBSPricePerGiBMonth(volumeType string) float64 {
	if p, ok := ebsPricePerGiBMonth[strings.ToLower(strings.TrimSpace(volumeType))]; ok {
		return p
	}
	return DefaultEBSPricePerGiBMonth
}

// VolumeTypeView is volume count and size for one EBS type.
type VolumeTypeView struct {
	VolumeType string
	Count      int
	SizeGiB    float64
}

// EBSTypeCost is the estimated monthly storage cost for one EBS volume type.
type EBSTypeCost struct {
	VolumeType          string  `json:"volume_type"`
	Count               int     `json:"count"`
	SizeGiB             float64 `json:"size_gib"`
	PricePerGiBMonth    float64 `json:"price_per_gib_month"`
	EstimatedMonthlyUSD float64 `json:"estimated_monthly_usd"`
}

// EstimateUnattachedEBS prices unattached volumes by type across accounts.
// Accounts that report a total size but no per-type breakdown (older caches,
// fixtures) have the untyped remainder priced at the default rate and
// reported under volume_type "unknown".
func EstimateUnattachedEBS(volumes []*VolumeView) (total float64, byType []EBSTypeCost) {
	agg := make(map[string]*EBSTypeCost)
	add := func(vt string, count int, gib float64) {
		if gib <= 0 && count == 0 {
			return
		}
		key := strings.ToLower(strings.TrimSpace(vt))
		if key == "" {
			key = "unknown"
		}
		c, ok := agg[key]
		if !ok {
			c = &EBSTypeCost{VolumeType: key, PricePerGiBMonth: EBSPricePerGiBMonth(key)}
			agg[key] = c
		}
		c.Count += count
		c.SizeGiB += gib
	}
	for _, v := range volumes {
		if v == nil || v.AvailableCount == 0 {
			continue
		}
		var typedGiB float64
		var typedCount int
		for _, t := range v.AvailableByType {
			add(t.VolumeType, t.Count, t.SizeGiB)
			typedGiB += t.SizeGiB
			typedCount += t.Count
		}
		if rest := v.AvailableGiB - typedGiB; rest > 0.0001 {
			restCount := v.AvailableCount - typedCount
			if restCount < 0 {
				restCount = 0
			}
			add("unknown", restCount, rest)
		}
	}
	for _, c := range agg {
		c.EstimatedMonthlyUSD = roundCents(c.SizeGiB * c.PricePerGiBMonth)
		total += c.SizeGiB * c.PricePerGiBMonth
		byType = append(byType, *c)
	}
	sort.Slice(byType, func(i, j int) bool {
		if byType[i].EstimatedMonthlyUSD != byType[j].EstimatedMonthlyUSD {
			return byType[i].EstimatedMonthlyUSD > byType[j].EstimatedMonthlyUSD
		}
		return byType[i].VolumeType < byType[j].VolumeType
	})
	return roundCents(total), byType
}

func roundCents(v float64) float64 {
	return math.Round(v*100) / 100
}
