package ec2snapshots

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type SnapshotSummary struct {
	AccountID      string    `json:"account_id"`
	AccountName    string    `json:"account_name"`
	Region         string    `json:"region"`
	Count          int       `json:"count"`
	TotalSizeGiB   float64   `json:"total_size_gib"`
	TotalSizeBytes int64     `json:"total_size_bytes"`
	ScannedAt      time.Time `json:"scanned_at"`
	ByTier         []TierBreakdown `json:"by_tier,omitempty"`
	RecentCreates  []SnapshotEvent `json:"recent_creates,omitempty"`
}

// TierBreakdown groups snapshots by storage tier (standard, archive, etc.).
type TierBreakdown struct {
	Tier    string  `json:"tier"`
	Count   int     `json:"count"`
	SizeGiB float64 `json:"size_gib"`
}

type SnapshotEvent struct {
	SnapshotID string    `json:"snapshot_id"`
	VolumeID   string    `json:"volume_id"`
	SizeGiB    float64   `json:"size_gib"`
	StartTime  time.Time `json:"start_time"`
	Description string   `json:"description,omitempty"`
}

func GetSnapshotSummary(ctx context.Context, client *ec2.Client, accountID, accountName, region string, recentLimit int) (*SnapshotSummary, error) {
	if recentLimit <= 0 {
		recentLimit = 10
	}

	var snapshots []types.Snapshot
	paginator := ec2.NewDescribeSnapshotsPaginator(client, &ec2.DescribeSnapshotsInput{
		OwnerIds: []string{"self"},
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe snapshots: %w", err)
		}
		snapshots = append(snapshots, page.Snapshots...)
	}

	byTier := make(map[string]*TierBreakdown)
	var totalGiB float64
	var recent []SnapshotEvent

	for _, s := range snapshots {
		volGiB := float64(aws.ToInt32(s.VolumeSize))
		totalGiB += volGiB

		tier := string(s.StorageTier)
		if tier == "" {
			tier = "standard"
		}
		if b, ok := byTier[tier]; ok {
			b.Count++
			b.SizeGiB += volGiB
		} else {
			byTier[tier] = &TierBreakdown{
				Tier:    tier,
				Count:   1,
				SizeGiB: volGiB,
			}
		}

		if s.StartTime != nil {
			recent = append(recent, SnapshotEvent{
				SnapshotID:  aws.ToString(s.SnapshotId),
				VolumeID:    aws.ToString(s.VolumeId),
				SizeGiB:     float64(aws.ToInt32(s.VolumeSize)),
				StartTime:   *s.StartTime,
				Description: aws.ToString(s.Description),
			})
		}
	}

	sort.Slice(recent, func(i, j int) bool {
		return recent[i].StartTime.After(recent[j].StartTime)
	})
	if len(recent) > recentLimit {
		recent = recent[:recentLimit]
	}

	breakdown := make([]TierBreakdown, 0, len(byTier))
	for _, b := range byTier {
		breakdown = append(breakdown, *b)
	}
	sort.Slice(breakdown, func(i, j int) bool { return breakdown[i].SizeGiB > breakdown[j].SizeGiB })

	return &SnapshotSummary{
		AccountID:      accountID,
		AccountName:    accountName,
		Region:         region,
		Count:          len(snapshots),
		TotalSizeGiB:   totalGiB,
		TotalSizeBytes: int64(totalGiB * 1024 * 1024 * 1024),
		ScannedAt:      time.Now().UTC(),
		ByTier:         breakdown,
		RecentCreates:  recent,
	}, nil
}
