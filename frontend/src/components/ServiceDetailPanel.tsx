import type { ServiceDetail } from "../types";
import { formatCurrency, formatServiceName } from "../utils/format";
import DailyCostChart from "./DailyCostChart";
import UsageTable from "./UsageTable";

interface Props {
  detail: ServiceDetail;
}

export default function ServiceDetailPanel({ detail }: Props) {
  const title = `${formatServiceName(detail.service)} detail`;
  const rawName =
    formatServiceName(detail.service) !== detail.service ? detail.service : null;

  return (
    <div className="category-section">
      <h3 className="category-subhead">{title}</h3>
      {rawName && (
        <p className="category-desc">
          Service: <span className="mono">{rawName}</span>
        </p>
      )}

      <div className="kpi-grid" style={{ marginBottom: "1rem" }}>
        <div className="kpi">
          <label>Total (usage)</label>
          <div className="value">{formatCurrency(detail.total)}</div>
          <div className="kpi-sub">
            {new Date(detail.start + "T00:00:00").toLocaleDateString()} –{" "}
            {new Date(detail.end + "T00:00:00").toLocaleDateString()}
          </div>
        </div>
      </div>

      <DailyCostChart data={detail.daily ?? []} title="Daily cost (usage-only)" />

      <div style={{ marginTop: "1rem" }}>
        <UsageTable
          title="By region"
          showPercent
          labelHeader="Region"
          rows={(detail.by_region ?? []).map((r) => ({
            label: r.key,
            amount: r.amount,
            percent: r.percent,
          }))}
        />
      </div>

      <div style={{ marginTop: "1rem" }}>
        <UsageTable
          title="By usage type"
          showPercent
          labelHeader="Usage type"
          rows={(detail.by_usage_type ?? []).map((r) => ({
            label: r.key,
            amount: r.amount,
            percent: r.percent,
          }))}
        />
      </div>

      <div style={{ marginTop: "1rem" }}>
        <UsageTable
          title="By operation"
          showPercent
          labelHeader="Operation"
          rows={(detail.by_operation ?? []).map((r) => ({
            label: r.key,
            amount: r.amount,
            percent: r.percent,
          }))}
        />
      </div>

      {detail.by_name_tag && detail.by_name_tag.length > 0 && (
        <div style={{ marginTop: "1rem" }}>
          <UsageTable
            title="Costs by tag Name"
            showPercent
            labelHeader="Tag Name value"
            rows={(detail.by_name_tag ?? []).map((r) => ({
              label: r.key,
              amount: r.amount,
              percent: r.percent,
            }))}
          />
        </div>
      )}
    </div>
  );
}

