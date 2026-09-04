import DailyCostChart from "./DailyCostChart";
import DailyCountChart from "./DailyCountChart";
import type { EBSSnapshotDetail } from "../types";
import { formatGiB } from "../utils/format";

interface Props {
  detail: EBSSnapshotDetail;
}

export default function EBSSnapshotPanel({ detail }: Props) {
  const inv = detail.inventory;

  return (
    <div className="category-section">
      <h3 className="category-subhead">EBS snapshot inventory &amp; daily cost</h3>
      {detail.inventory_note && (
        <p className="category-desc">{detail.inventory_note}</p>
      )}

      {inv && (
        <>
          <div className="kpi-grid" style={{ marginBottom: "1rem" }}>
            <div className="kpi">
              <label>Snapshots now (EC2 API)</label>
              <div className="value">{inv.count.toLocaleString()}</div>
            </div>
            <div className="kpi">
              <label>Total snapshot size</label>
              <div className="value">{formatGiB(inv.total_size_gib)}</div>
            </div>
          </div>

          {inv.by_tier && inv.by_tier.length > 0 && (
            <div className="table-wrap" style={{ marginBottom: "1rem" }}>
              <table>
                <thead>
                  <tr>
                    <th>Storage tier</th>
                    <th className="amount">Count</th>
                    <th className="amount">Size</th>
                  </tr>
                </thead>
                <tbody>
                  {inv.by_tier.map((t) => (
                    <tr key={t.tier}>
                      <td className="mono">{t.tier}</td>
                      <td className="amount">{t.count}</td>
                      <td className="amount">{formatGiB(t.size_gib)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {detail.cur_daily && detail.cur_daily.length > 0 && (
        <DailyCountChart
          data={detail.cur_daily}
          title="Daily snapshot count (CUR — distinct snapshots billed)"
          countLabel="Snapshots"
          color="#b5179e"
        />
      )}

      {(detail.daily_cost?.length ?? 0) > 0 && (
        <DailyCostChart
          data={detail.daily_cost ?? []}
          title="Daily EBS snapshot cost (storage + API)"
        />
      )}
    </div>
  );
}
