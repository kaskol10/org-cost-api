package analysis

import "sort"

type TagDelta struct {
	Key              string  `json:"key"`
	CurrentUSD       float64 `json:"current_usd"`
	PriorUSD         float64 `json:"prior_usd"`
	ChangeUSD        float64 `json:"change_usd"`
	ChangePercent    float64 `json:"change_percent"`
	CurrentSharePct  float64 `json:"current_share_pct,omitempty"`
}

type AccountTagDelta struct {
	AccountID   string     `json:"account_id"`
	AccountName string     `json:"account_name"`
	TotalCurrentUSD float64 `json:"total_current_usd"`
	TotalPriorUSD   float64 `json:"total_prior_usd"`
	DeltaUSD        float64 `json:"delta_usd"`
	DeltaPercent    float64 `json:"delta_percent"`
	TopIncreases    []TagDelta `json:"top_increases"`
	TopDecreases    []TagDelta `json:"top_decreases"`
}

type ServiceTagDeltaResponse struct {
	GeneratedAt    string           `json:"generated_at"`
	Service        string           `json:"service"`
	DisplayName    string           `json:"display_name"`
	TagKey         string           `json:"tag_key"`
	CurrentPeriod  PeriodSummary    `json:"current_period"`
	PriorPeriod    PeriodSummary    `json:"prior_period"`
	TotalCurrentUSD float64         `json:"total_current_usd"`
	TotalPriorUSD   float64         `json:"total_prior_usd"`
	DeltaUSD        float64         `json:"delta_usd"`
	DeltaPercent    float64         `json:"delta_percent"`
	TopIncreases    []TagDelta      `json:"top_increases"`
	TopDecreases    []TagDelta      `json:"top_decreases"`
	Accounts        []AccountTagDelta `json:"accounts"`
	TopAccountNote  string          `json:"top_account_note,omitempty"`
	CECallsUsed     int             `json:"ce_calls_used"`
}

const minTagDeltaUSD = 1.0

// BuildTagDeltaDrivers computes top increases/decreases by absolute USD delta for tag buckets.
func BuildTagDeltaDrivers(cur, prior map[string]float64, totalCurrent float64, limit int) (increases []TagDelta, decreases []TagDelta) {
	// Keep slices non-nil for JSON stability (avoid `null` in responses).
	increases = []TagDelta{}
	decreases = []TagDelta{}

	all := make(map[string]struct{}, len(cur)+len(prior))
	for k := range cur {
		all[k] = struct{}{}
	}
	for k := range prior {
		all[k] = struct{}{}
	}

	for k := range all {
		curAmt := cur[k]
		priorAmt := prior[k]
		if curAmt <= 0 && priorAmt <= 0 {
			continue
		}

		changeUSD := curAmt - priorAmt
		if changeUSD >= minTagDeltaUSD {
			cp := 0.0
			if priorAmt > 0 {
				cp = (changeUSD / priorAmt) * 100
			} else {
				// New tag bucket (no prior).
				cp = 100
			}
			share := 0.0
			if totalCurrent > 0 {
				share = (curAmt / totalCurrent) * 100
			}
			increases = append(increases, TagDelta{
				Key:             k,
				CurrentUSD:      curAmt,
				PriorUSD:        priorAmt,
				ChangeUSD:       changeUSD,
				ChangePercent:   cp,
				CurrentSharePct: share,
			})
		} else if -changeUSD >= minTagDeltaUSD {
			cp := 0.0
			if priorAmt > 0 {
				cp = (changeUSD / priorAmt) * 100
			} else {
				// Prior is ~0 but we have a negative change shouldn't happen; keep 0%.
				cp = 0
			}
			decreases = append(decreases, TagDelta{
				Key:            k,
				CurrentUSD:     curAmt,
				PriorUSD:       priorAmt,
				ChangeUSD:      changeUSD,
				ChangePercent:  cp,
				CurrentSharePct: func() float64 {
					if totalCurrent > 0 {
						return (curAmt / totalCurrent) * 100
					}
					return 0
				}(),
			})
		}
	}

	sort.Slice(increases, func(i, j int) bool { return increases[i].ChangeUSD > increases[j].ChangeUSD })
	sort.Slice(decreases, func(i, j int) bool { return decreases[i].ChangeUSD < decreases[j].ChangeUSD })

	if limit > 0 && len(increases) > limit {
		increases = increases[:limit]
	}
	if limit > 0 && len(decreases) > limit {
		decreases = decreases[:limit]
	}
	return increases, decreases
}

// FriendlyService is a shared label normalizer for UI and explanations.
func FriendlyService(service string) string {
	return friendlyService(service)
}

