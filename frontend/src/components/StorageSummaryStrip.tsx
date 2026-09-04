import type { EBSVolumeInventory, SnapshotSummary } from "../types";
import { formatCountSize, formatGiB, formatPercent } from "../utils/format";

interface Props {
  volumes?: EBSVolumeInventory | null;
  snapshots?: SnapshotSummary | null;
  volumesNote?: string;
  snapshotsNote?: string;
}

export default function StorageSummaryStrip({
  volumes,
  snapshots,
  volumesNote,
  snapshotsNote,
}: Props) {
  const hasVolumes = volumes && volumes.count >= 0;
  const hasSnapshots = snapshots && snapshots.count >= 0;

  if (!hasVolumes && !hasSnapshots) {
    return null;
  }

  return (
    <section className="storage-summary">
      <h2 className="storage-summary-title">EBS storage & waste signals (live inventory)</h2>
      <div className="kpi-grid storage-kpi-grid">
        {hasVolumes && (
          <>
            <div className="kpi kpi-storage">
              <label>EBS disks</label>
              <div className="value">{volumes!.count.toLocaleString()}</div>
              <div className="kpi-sub">{formatGiB(volumes!.total_gib)} provisioned</div>
            </div>
            {volumes!.usage?.filesystem_used_gib != null &&
              volumes!.usage.utilization_percent != null && (
                <div className="kpi kpi-storage">
                  <label>Filesystem used</label>
                  <div className="value">
                    {formatGiB(volumes!.usage.filesystem_used_gib)}
                  </div>
                  <div className="kpi-sub">
                    {formatPercent(volumes!.usage.utilization_percent)} of measured EBS
                    {volumes!.usage.coverage_percent < 100 &&
                      ` · ${formatPercent(volumes!.usage.coverage_percent)} coverage`}
                  </div>
                </div>
              )}
            {(volumes!.available_count ?? 0) > 0 && (
              <div className="kpi kpi-storage">
                <label>Unattached disks</label>
                <div className="value">{(volumes!.available_count ?? 0).toLocaleString()}</div>
                <div className="kpi-sub">
                  {formatGiB(volumes!.available_gib ?? 0)} provisioned · status=available
                </div>
              </div>
            )}
            {volumes!.by_type?.length > 0 && (
              <div className="kpi kpi-storage kpi-wide">
                <label>By volume type</label>
                <ul className="storage-type-list">
                  {volumes!.by_type.map((t) => (
                    <li key={t.volume_type}>
                      <span className="mono">{t.volume_type}</span>
                      <span>
                        {formatCountSize(t.count, t.size_gib, t.count === 1 ? "disk" : "disks")}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
        {hasSnapshots && (
          <>
            <div className="kpi kpi-storage">
              <label>EBS snapshots</label>
              <div className="value">{snapshots!.count.toLocaleString()}</div>
              <div className="kpi-sub">{formatGiB(snapshots!.total_size_gib)} stored</div>
            </div>
            {snapshots!.by_tier && snapshots!.by_tier.length > 0 && (
              <div className="kpi kpi-storage kpi-wide">
                <label>By storage tier</label>
                <ul className="storage-type-list">
                  {snapshots!.by_tier.map((t) => (
                    <li key={t.tier}>
                      <span className="mono">{t.tier}</span>
                      <span>
                        {formatCountSize(t.count, t.size_gib, t.count === 1 ? "snap" : "snaps")}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
      </div>
      {(volumesNote || snapshotsNote) && (
        <p className="category-desc storage-summary-note">
          {[volumesNote, snapshotsNote].filter(Boolean).join(" ")}
        </p>
      )}
    </section>
  );
}
