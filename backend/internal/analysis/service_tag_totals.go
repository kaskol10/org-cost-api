package analysis

import "sort"

type TagBucket struct {
	Key              string  `json:"key"`
	CurrentUSD       float64 `json:"current_usd"`
	CurrentSharePct float64 `json:"current_share_pct,omitempty"`
}

type ServiceTagTotalsResponse struct {
	GeneratedAt    string        `json:"generated_at"`
	Service        string        `json:"service"`
	DisplayName    string        `json:"display_name"`
	TagKey         string        `json:"tag_key"`
	CurrentPeriod  PeriodSummary `json:"current_period"`
	TotalCurrentUSD float64      `json:"total_current_usd"`
	Buckets        []TagBucket   `json:"buckets"`
	CECallsUsed    int           `json:"ce_calls_used"`
}

func TopBucketsByAmount(amounts map[string]float64, total float64, limit int) []TagBucket {
	type kv struct {
		k string
		v float64
	}
	var list []kv
	for k, v := range amounts {
		if v <= 0 {
			continue
		}
		list = append(list, kv{k: k, v: v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v })
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	out := make([]TagBucket, 0, len(list))
	for _, item := range list {
		share := 0.0
		if total > 0 {
			share = (item.v / total) * 100
		}
		out = append(out, TagBucket{
			Key:              item.k,
			CurrentUSD:       item.v,
			CurrentSharePct: share,
		})
	}
	if out == nil {
		out = []TagBucket{}
	}
	return out
}

