package costexplorer

import (
	"github.com/kaskol10/org-cost-api/backend/internal/aws/cur"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/ec2snapshots"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/ec2volumes"
)

// DailyResourceInventory is distinct billed resources per day (from CUR via Athena).
type DailyResourceInventory struct {
	Date        string  `json:"date"`
	Count       int     `json:"count"`
	UsageAmount float64 `json:"usage_amount,omitempty"`
}

// StorageContext holds EC2 inventory fetched once per account (avoids duplicate API calls).
type StorageContext struct {
	Volumes         *ec2volumes.Inventory
	Snapshots       *ec2snapshots.SnapshotSummary
	CURVolumesDaily []DailyResourceInventory
	CURSnapshotsDaily []DailyResourceInventory
}

// NewStorageContext builds per-account storage context including optional CUR daily series.
func NewStorageContext(
	vols *ec2volumes.Inventory,
	snaps *ec2snapshots.SnapshotSummary,
	curInv *cur.AccountDailyInventory,
	accountID string,
) *StorageContext {
	sc := &StorageContext{Volumes: vols, Snapshots: snaps}
	if curInv != nil {
		sc.CURVolumesDaily = dailyFromCUR(curInv.Volumes[accountID])
		sc.CURSnapshotsDaily = dailyFromCUR(curInv.Snapshots[accountID])
	}
	return sc
}

func dailyFromCUR(rows []cur.DailyResourceCount) []DailyResourceInventory {
	if len(rows) == 0 {
		return nil
	}
	out := make([]DailyResourceInventory, len(rows))
	for i, r := range rows {
		out[i] = DailyResourceInventory{
			Date:        r.Date,
			Count:       r.Count,
			UsageAmount: r.UsageAmount,
		}
	}
	return out
}

// EBSSnapshotDetail combines live snapshot inventory with daily snapshot cost.
type EBSSnapshotDetail struct {
	Inventory     *SnapshotInventoryView     `json:"inventory,omitempty"`
	DailyCost     []DailyCost                `json:"daily_cost,omitempty"`
	CURDaily      []DailyResourceInventory   `json:"cur_daily,omitempty"`
	InventoryNote string                     `json:"inventory_note,omitempty"`
}

// SnapshotInventoryView is count/size for the snapshots category panel.
type SnapshotInventoryView struct {
	Count        int                           `json:"count"`
	TotalSizeGiB float64                       `json:"total_size_gib"`
	ByTier       []ec2snapshots.TierBreakdown  `json:"by_tier,omitempty"`
}

func snapshotInventoryView(s *ec2snapshots.SnapshotSummary) *SnapshotInventoryView {
	if s == nil {
		return nil
	}
	return &SnapshotInventoryView{
		Count:        s.Count,
		TotalSizeGiB: s.TotalSizeGiB,
		ByTier:       s.ByTier,
	}
}
