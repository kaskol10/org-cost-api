package ec2volumes

import (
	"context"
	"math"
	"sort"

	cwdisk "github.com/kaskol10/org-cost-api/backend/internal/aws/cloudwatch"
	"github.com/kaskol10/org-cost-api/backend/internal/errmsg"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

type volumeRecord struct {
	ID         string
	SizeGiB    float64
	VolumeType string
	State      string
	InstanceID string
}

// InstanceUsage is filesystem used vs EBS provisioned for one EC2 instance.
type InstanceUsage struct {
	InstanceID           string   `json:"instance_id"`
	VolumeCount          int      `json:"volume_count"`
	ProvisionedGiB       float64  `json:"provisioned_gib"`
	FilesystemUsedGiB    *float64 `json:"filesystem_used_gib,omitempty"`
	FilesystemTotalGiB   *float64 `json:"filesystem_total_gib,omitempty"`
	UtilizationPercent   *float64 `json:"utilization_percent,omitempty"`
}

// VolumeUsageRow highlights volumes with low utilization or no attachment.
type VolumeUsageRow struct {
	VolumeID            string   `json:"volume_id"`
	InstanceID          string   `json:"instance_id,omitempty"`
	VolumeType          string   `json:"volume_type"`
	State               string   `json:"state"`
	ProvisionedGiB      float64  `json:"provisioned_gib"`
	FilesystemUsedGiB   *float64 `json:"filesystem_used_gib,omitempty"`
	UtilizationPercent  *float64 `json:"utilization_percent,omitempty"`
	Note                string   `json:"note,omitempty"`
}

// UsageSummary compares provisioned EBS capacity with filesystem usage (CloudWatch agent).
type UsageSummary struct {
	ProvisionedGiB           float64           `json:"provisioned_gib"`
	AttachedProvisionedGiB   float64           `json:"attached_provisioned_gib"`
	UnattachedProvisionedGiB float64           `json:"unattached_provisioned_gib"`
	FilesystemUsedGiB        *float64          `json:"filesystem_used_gib,omitempty"`
	UtilizationPercent       *float64          `json:"utilization_percent,omitempty"`
	MeasuredProvisionedGiB   float64           `json:"measured_provisioned_gib"`
	CoveragePercent          float64           `json:"coverage_percent"`
	UsageNote                string            `json:"usage_note,omitempty"`
	ByInstance               []InstanceUsage   `json:"by_instance,omitempty"`
	TopUnderutilized         []VolumeUsageRow  `json:"top_underutilized,omitempty"`
}

func enrichUsage(ctx context.Context, inv *Inventory, volumes []volumeRecord, cw *cloudwatch.Client) {
	if inv == nil || len(volumes) == 0 {
		return
	}

	instanceVolumes := make(map[string][]volumeRecord)
	var instanceIDs []string
	seen := make(map[string]struct{})
	for _, v := range volumes {
		if v.State == "available" {
			continue
		}
		if v.InstanceID == "" {
			continue
		}
		instanceVolumes[v.InstanceID] = append(instanceVolumes[v.InstanceID], v)
		if _, ok := seen[v.InstanceID]; !ok {
			seen[v.InstanceID] = struct{}{}
			instanceIDs = append(instanceIDs, v.InstanceID)
		}
	}

	diskByInstance, err := cwdisk.FetchInstanceDiskUsage(ctx, cw, instanceIDs)
	if err != nil {
		inv.Usage = &UsageSummary{
			ProvisionedGiB: inv.TotalGiB,
			UsageNote:      errmsg.Note("Could not read CloudWatch agent disk metrics", err),
		}
		fillProvisionedBreakdown(inv.Usage, volumes)
		return
	}

	summary := &UsageSummary{ProvisionedGiB: inv.TotalGiB}
	fillProvisionedBreakdown(summary, volumes)

	var measuredUsedGiB, measuredProvisionedGiB float64
	var byInstance []InstanceUsage
	var candidates []VolumeUsageRow

	for _, v := range volumes {
		if v.State == "available" {
			zero := 0.0
			candidates = append(candidates, VolumeUsageRow{
				VolumeID:           v.ID,
				VolumeType:         v.VolumeType,
				State:              v.State,
				ProvisionedGiB:     v.SizeGiB,
				FilesystemUsedGiB:  &zero,
				UtilizationPercent: &zero,
				Note:               "Unattached — paying for full provisioned size",
			})
			measuredProvisionedGiB += v.SizeGiB
			continue
		}
	}

	for instanceID, vols := range instanceVolumes {
		var provGiB float64
		for _, v := range vols {
			provGiB += v.SizeGiB
		}
		inst := InstanceUsage{
			InstanceID:     instanceID,
			VolumeCount:    len(vols),
			ProvisionedGiB: provGiB,
		}

		if disk, ok := diskByInstance[instanceID]; ok && (disk.UsedBytes > 0 || disk.TotalBytes > 0) {
			usedGiB := bytesToGiB(disk.UsedBytes)
			totalGiB := bytesToGiB(disk.TotalBytes)
			inst.FilesystemUsedGiB = &usedGiB
			if totalGiB > 0 {
				inst.FilesystemTotalGiB = &totalGiB
			}
			util := usedGiB / provGiB * 100
			if provGiB > 0 {
				inst.UtilizationPercent = &util
			}

			measuredUsedGiB += usedGiB
			measuredProvisionedGiB += provGiB

			if len(vols) == 1 {
				v := vols[0]
				volUtil := math.Min(100, usedGiB/v.SizeGiB*100)
				candidates = append(candidates, VolumeUsageRow{
					VolumeID:           v.ID,
					InstanceID:         instanceID,
					VolumeType:         v.VolumeType,
					State:              v.State,
					ProvisionedGiB:     v.SizeGiB,
					FilesystemUsedGiB:  &usedGiB,
					UtilizationPercent: &volUtil,
				})
			}
		} else if len(vols) == 1 {
			candidates = append(candidates, VolumeUsageRow{
				VolumeID:       vols[0].ID,
				InstanceID:     instanceID,
				VolumeType:     vols[0].VolumeType,
				State:          vols[0].State,
				ProvisionedGiB: vols[0].SizeGiB,
				Note:           "No CloudWatch agent disk metrics for this instance",
			})
		}
		byInstance = append(byInstance, inst)
	}

	sort.Slice(byInstance, func(i, j int) bool {
		return byInstance[i].ProvisionedGiB > byInstance[j].ProvisionedGiB
	})
	if len(byInstance) > 25 {
		byInstance = byInstance[:25]
	}
	summary.ByInstance = byInstance

	summary.MeasuredProvisionedGiB = measuredProvisionedGiB
	if inv.TotalGiB > 0 {
		summary.CoveragePercent = measuredProvisionedGiB / inv.TotalGiB * 100
	}
	if measuredProvisionedGiB > 0 && measuredUsedGiB > 0 {
		used := measuredUsedGiB
		summary.FilesystemUsedGiB = &used
		util := measuredUsedGiB / measuredProvisionedGiB * 100
		summary.UtilizationPercent = &util
	}

	summary.TopUnderutilized = pickTopUnderutilized(candidates, 20)
	summary.UsageNote = usageNote(summary.CoveragePercent, len(diskByInstance) > 0)
	inv.Usage = summary
}

func fillProvisionedBreakdown(summary *UsageSummary, volumes []volumeRecord) {
	for _, v := range volumes {
		if v.State == "available" {
			summary.UnattachedProvisionedGiB += v.SizeGiB
		} else {
			summary.AttachedProvisionedGiB += v.SizeGiB
		}
	}
}

func usageNote(coveragePercent float64, anyMetrics bool) string {
	if !anyMetrics && coveragePercent < 1 {
		return "No CloudWatch agent (CWAgent) disk metrics found. EC2 only reports provisioned size. " +
			"Install the CloudWatch agent on instances to compare filesystem used vs provisioned EBS."
	}
	if coveragePercent < 100 {
		return "Filesystem usage is from the CloudWatch agent where installed. " +
			"Utilization covers attached single-volume instances and all unattached disks. " +
			"Multi-volume instances show per-instance totals in the table below."
	}
	return "Filesystem usage from CloudWatch agent (CWAgent), summed across mount points per instance."
}

func pickTopUnderutilized(rows []VolumeUsageRow, limit int) []VolumeUsageRow {
	sort.Slice(rows, func(i, j int) bool {
		// Unattached first, then lowest utilization, then largest provisioned.
		if rows[i].State == "available" && rows[j].State != "available" {
			return true
		}
		if rows[i].State != "available" && rows[j].State == "available" {
			return false
		}
		ui, uj := utilOrInf(rows[i]), utilOrInf(rows[j])
		if ui != uj {
			return ui < uj
		}
		return rows[i].ProvisionedGiB > rows[j].ProvisionedGiB
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

func utilOrInf(row VolumeUsageRow) float64 {
	if row.UtilizationPercent != nil {
		return *row.UtilizationPercent
	}
	return 999
}

func bytesToGiB(b float64) float64 {
	return b / (1024 * 1024 * 1024)
}
