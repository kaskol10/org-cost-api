package ec2volumes

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type TypeBreakdown struct {
	VolumeType string  `json:"volume_type"`
	Count      int     `json:"count"`
	SizeGiB    float64 `json:"size_gib"`
}

type Inventory struct {
	Count          int             `json:"count"`
	AvailableCount int             `json:"available_count"` // unattached (status=available)
	AvailableGiB   float64         `json:"available_gib"`   // provisioned size of unattached volumes only
	TotalGiB       float64         `json:"total_gib"`
	ByType         []TypeBreakdown `json:"by_type"`
	Usage          *UsageSummary   `json:"usage,omitempty"`
	ScannedAt      string          `json:"scanned_at,omitempty"`
}

// ListInventory returns EBS volumes and optional filesystem usage (CloudWatch agent).
func ListInventory(ctx context.Context, client *ec2.Client, cw *cloudwatch.Client) (*Inventory, error) {
	byType := make(map[string]*TypeBreakdown)
	var totalGiB, availableGiB float64
	var count, availableCount int
	var records []volumeRecord

	paginator := ec2.NewDescribeVolumesPaginator(client, &ec2.DescribeVolumesInput{
		Filters: []types.Filter{{
			Name:   aws.String("status"),
			Values: []string{"available", "in-use", "creating", "deleting", "error"},
		}},
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe volumes: %w", err)
		}
		for _, vol := range page.Volumes {
			count++
			state := string(vol.State)
			gib := float64(aws.ToInt32(vol.Size))
			if vol.State == types.VolumeStateAvailable {
				availableCount++
				availableGiB += gib
			}
			totalGiB += gib
			vt := string(vol.VolumeType)
			if vt == "" {
				vt = "unknown"
			}
			if b, ok := byType[vt]; ok {
				b.Count++
				b.SizeGiB += gib
			} else {
				byType[vt] = &TypeBreakdown{VolumeType: vt, Count: 1, SizeGiB: gib}
			}

			rec := volumeRecord{
				ID:         aws.ToString(vol.VolumeId),
				SizeGiB:    gib,
				VolumeType: vt,
				State:      state,
			}
			if len(vol.Attachments) > 0 {
				rec.InstanceID = aws.ToString(vol.Attachments[0].InstanceId)
			}
			records = append(records, rec)
		}
	}

	breakdown := make([]TypeBreakdown, 0, len(byType))
	for _, b := range byType {
		breakdown = append(breakdown, *b)
	}
	sort.Slice(breakdown, func(i, j int) bool { return breakdown[i].SizeGiB > breakdown[j].SizeGiB })

	inv := &Inventory{
		Count:          count,
		AvailableCount: availableCount,
		AvailableGiB:   availableGiB,
		TotalGiB:       totalGiB,
		ByType:         breakdown,
	}
	enrichUsage(ctx, inv, records, cw)
	return inv, nil
}
