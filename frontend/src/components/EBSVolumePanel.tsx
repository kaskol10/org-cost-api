import DailyCostChart from "./DailyCostChart";
import DailyCountChart from "./DailyCountChart";
import type { EBSVolumeDetail } from "../types";
import { formatGiB, formatPercent } from "../utils/format";

interface Props {
  detail: EBSVolumeDetail;
}

function UtilBar({ percent }: { percent: number }) {
  const clamped = Math.min(100, Math.max(0, percent));
  const tone =
    clamped < 30 ? "util-low" : clamped < 60 ? "util-mid" : "util-high";
  return (
    <div className="util-bar" title={`${clamped.toFixed(0)}% used`}>
      <div className={`util-bar-fill ${tone}`} style={{ width: `${clamped}%` }} />
    </div>
  );
}

export default function EBSVolumePanel({ detail }: Props) {
  const inv = detail.inventory;
  const usage = inv?.usage;

  return (
    <div className="category-section">
      <h3 className="category-subhead">EBS volume inventory &amp; daily cost</h3>
      {detail.inventory_note && (
        <p className="category-desc">{detail.inventory_note}</p>
      )}

      {inv && (
        <div className="kpi-grid" style={{ marginBottom: "1rem" }}>
          <div className="kpi">
            <label>Disks now (EC2 API)</label>
            <div className="value">{inv.count}</div>
          </div>
          <div className="kpi">
            <label>Total provisioned size</label>
            <div className="value">{formatGiB(inv.total_gib)}</div>
          </div>
          {usage?.filesystem_used_gib != null && usage.utilization_percent != null && (
            <>
              <div className="kpi">
                <label>Filesystem used (agent)</label>
                <div className="value">{formatGiB(usage.filesystem_used_gib)}</div>
                <div className="kpi-sub">
                  {formatPercent(usage.utilization_percent)} of {formatGiB(usage.measured_provisioned_gib)} measured
                </div>
              </div>
              <div className="kpi">
                <label>Usage coverage</label>
                <div className="value">{formatPercent(usage.coverage_percent)}</div>
                <div className="kpi-sub">of provisioned GiB has agent data</div>
              </div>
            </>
          )}
        </div>
      )}

      {usage?.usage_note && (
        <p className="category-desc" style={{ marginBottom: "1rem" }}>
          {usage.usage_note}
        </p>
      )}

      {usage && usage.unattached_provisioned_gib > 0 && (
        <p className="category-desc" style={{ marginBottom: "1rem" }}>
          Unattached disks: {formatGiB(usage.unattached_provisioned_gib)} provisioned with no filesystem use — strong cleanup candidates.
        </p>
      )}

      {usage?.top_underutilized && usage.top_underutilized.length > 0 && (
        <div className="table-wrap" style={{ marginBottom: "1rem" }}>
          <h4 className="category-subhead" style={{ fontSize: "0.95rem" }}>
            Top underutilized volumes
          </h4>
          <table>
            <thead>
              <tr>
                <th>Volume</th>
                <th>Type</th>
                <th className="amount">Provisioned</th>
                <th className="amount">Used</th>
                <th>Utilization</th>
              </tr>
            </thead>
            <tbody>
              {usage.top_underutilized.map((row) => (
                <tr key={row.volume_id}>
                  <td className="mono">
                    {row.volume_id}
                    {row.instance_id && (
                      <div className="kpi-sub">{row.instance_id}</div>
                    )}
                    {row.note && <div className="kpi-sub">{row.note}</div>}
                  </td>
                  <td className="mono">{row.volume_type}</td>
                  <td className="amount">{formatGiB(row.provisioned_gib)}</td>
                  <td className="amount">
                    {row.filesystem_used_gib != null
                      ? formatGiB(row.filesystem_used_gib)
                      : "—"}
                  </td>
                  <td>
                    {row.utilization_percent != null ? (
                      <UtilBar percent={row.utilization_percent} />
                    ) : (
                      "—"
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {usage?.by_instance && usage.by_instance.length > 0 && (
        <div className="table-wrap" style={{ marginBottom: "1rem" }}>
          <h4 className="category-subhead" style={{ fontSize: "0.95rem" }}>
            By instance (multi-volume hosts)
          </h4>
          <table>
            <thead>
              <tr>
                <th>Instance</th>
                <th className="amount">Volumes</th>
                <th className="amount">EBS provisioned</th>
                <th className="amount">Filesystem used</th>
                <th>Utilization</th>
              </tr>
            </thead>
            <tbody>
              {usage.by_instance
                .filter((i) => i.volume_count > 1)
                .map((row) => (
                  <tr key={row.instance_id}>
                    <td className="mono">{row.instance_id}</td>
                    <td className="amount">{row.volume_count}</td>
                    <td className="amount">{formatGiB(row.provisioned_gib)}</td>
                    <td className="amount">
                      {row.filesystem_used_gib != null
                        ? formatGiB(row.filesystem_used_gib)
                        : "—"}
                    </td>
                    <td>
                      {row.utilization_percent != null ? (
                        <UtilBar percent={row.utilization_percent} />
                      ) : (
                        "—"
                      )}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      )}

      {inv?.by_type && inv.by_type.length > 0 && (
        <div className="table-wrap" style={{ marginBottom: "1rem" }}>
          <table>
            <thead>
              <tr>
                <th>Volume type</th>
                <th className="amount">Count</th>
                <th className="amount">Size</th>
              </tr>
            </thead>
            <tbody>
              {inv.by_type.map((t) => (
                <tr key={t.volume_type}>
                  <td className="mono">{t.volume_type}</td>
                  <td className="amount">{t.count}</td>
                  <td className="amount">{formatGiB(t.size_gib)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {detail.cur_daily && detail.cur_daily.length > 0 && (
        <DailyCountChart
          data={detail.cur_daily}
          title="Daily disk count (CUR — distinct volumes billed)"
          countLabel="Volumes"
          color="#4cc9f0"
        />
      )}

      <DailyCostChart
        data={detail.daily_cost ?? []}
        title="Daily EBS volume cost (storage + provisioned IOPS/throughput)"
      />
    </div>
  );
}
