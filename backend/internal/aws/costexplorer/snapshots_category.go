package costexplorer

import (
	"context"

	ce "github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/kaskol10/org-cost-api/backend/internal/aws/ec2snapshots"
	"github.com/kaskol10/org-cost-api/backend/internal/errmsg"
)

func enrichEBSSnapshotsCategory(
	ctx context.Context,
	ceClient *ce.Client,
	ec2Client *ec2.Client,
	accountID, linkedAccountID, accountName, region, start, end string,
	cat *UsageCategory,
	byUsage map[string]float64,
	storage *StorageContext,
) error {
	keys := usageTypeKeysForCategory(byUsage, "ebs-snapshots")
	if len(keys) == 0 {
		return nil
	}

	var snaps *ec2snapshots.SnapshotSummary
	var snapErr error
	if storage != nil && storage.Snapshots != nil {
		snaps = storage.Snapshots
	} else {
		snaps, snapErr = ec2snapshots.GetSnapshotSummary(ctx, ec2Client, accountID, accountName, region, 0)
	}
	if snapErr != nil {
		cat.EBSSnapshotDetail = &EBSSnapshotDetail{
			InventoryNote: errmsg.Note("Could not list EBS snapshots", snapErr),
		}
	} else if snaps != nil {
		note := "Snapshots (now): live DescribeSnapshots count and GiB (owner=self)."
		if storage != nil && len(storage.CURSnapshotsDaily) > 0 {
			note += " Daily snapshot count uses CUR (distinct snap-* billed each day)."
		} else {
			note += " Enable cur in config for per-day snapshot counts from CUR."
		}
		cat.EBSSnapshotDetail = &EBSSnapshotDetail{
			Inventory:     snapshotInventoryView(snaps),
			InventoryNote: note,
		}
		cat.InventoryCount = snaps.Count
		cat.InventorySizeGiB = snaps.TotalSizeGiB
	}
	if storage != nil && len(storage.CURSnapshotsDaily) > 0 {
		if cat.EBSSnapshotDetail == nil {
			cat.EBSSnapshotDetail = &EBSSnapshotDetail{}
		}
		cat.EBSSnapshotDetail.CURDaily = storage.CURSnapshotsDaily
	}

	filter := filterForUsageTypes(linkedAccountID, keys)
	daily, _, err := getDailyCosts(ctx, ceClient, start, end, filter)
	if err == nil {
		if cat.EBSSnapshotDetail == nil {
			cat.EBSSnapshotDetail = &EBSSnapshotDetail{}
		}
		cat.EBSSnapshotDetail.DailyCost = daily
	}

	ops, err := getGroupedCosts(ctx, ceClient, start, end, filter, types.GroupDefinitionTypeDimension, string(types.DimensionOperation))
	if err == nil {
		cat.APIOperations = toAPIOpCosts(ops, cat.Amount)
	}
	return nil
}
