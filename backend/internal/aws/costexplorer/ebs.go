package costexplorer

import (
	"context"
	"fmt"

	ce "github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/kaskol10/org-cost-api/backend/internal/aws/ec2volumes"
	"github.com/kaskol10/org-cost-api/backend/internal/errmsg"
)

// EBSVolumeDetail combines live volume inventory with daily storage cost.
type EBSVolumeDetail struct {
	Inventory       ec2volumes.Inventory     `json:"inventory"`
	DailyCost       []DailyCost              `json:"daily_cost"`
	CURDaily        []DailyResourceInventory `json:"cur_daily,omitempty"`
	InventoryNote   string                   `json:"inventory_note"`
}

func enrichEBSVolumesCategory(
	ctx context.Context,
	ceClient *ce.Client,
	ec2Client *ec2.Client,
	linkedAccountID, start, end string,
	cat *UsageCategory,
	byUsage map[string]float64,
	storage *StorageContext,
) error {
	keys := usageTypeKeysForCategory(byUsage, "ebs-volumes")
	if len(keys) == 0 {
		return nil
	}

	var inv *ec2volumes.Inventory
	var invErr error
	if storage != nil && storage.Volumes != nil {
		inv = storage.Volumes
	} else {
		inv, invErr = ec2volumes.ListInventory(ctx, ec2Client, nil)
	}
	if invErr != nil {
		cat.EBSVolumeDetail = &EBSVolumeDetail{
			InventoryNote: errmsg.Note("Could not list EBS volumes", invErr),
		}
	} else if inv != nil {
		note := "Disks (now): live DescribeVolumes count and provisioned GiB."
		if inv.Usage != nil && inv.Usage.UtilizationPercent != nil {
			note += fmt.Sprintf(" Filesystem utilization %.0f%% where CloudWatch agent reports (%.0f%% of provisioned GiB covered).",
				*inv.Usage.UtilizationPercent, inv.Usage.CoveragePercent)
		}
		if storage != nil && len(storage.CURVolumesDaily) > 0 {
			note += " Daily disk count chart uses CUR (distinct vol-* billed each day), including volumes since deleted."
		} else {
			note += " Enable cur in config for per-day disk counts from CUR. Daily cost is billed storage from Cost Explorer."
		}
		cat.EBSVolumeDetail = &EBSVolumeDetail{
			Inventory:     *inv,
			InventoryNote: note,
		}
		cat.InventoryCount = inv.Count
		cat.InventorySizeGiB = inv.TotalGiB
	}
	if storage != nil && len(storage.CURVolumesDaily) > 0 {
		if cat.EBSVolumeDetail == nil {
			cat.EBSVolumeDetail = &EBSVolumeDetail{}
		}
		cat.EBSVolumeDetail.CURDaily = storage.CURVolumesDaily
	}

	filter := filterForUsageTypes(linkedAccountID, keys)
	daily, _, err := getDailyCosts(ctx, ceClient, start, end, filter)
	if err == nil && cat.EBSVolumeDetail != nil {
		cat.EBSVolumeDetail.DailyCost = daily
	} else if cat.EBSVolumeDetail == nil {
		cat.EBSVolumeDetail = &EBSVolumeDetail{DailyCost: daily}
	}

	ops, err := getGroupedCosts(ctx, ceClient, start, end, filter, types.GroupDefinitionTypeDimension, string(types.DimensionOperation))
	if err == nil {
		cat.APIOperations = toAPIOpCosts(ops, cat.Amount)
	}
	return nil
}
