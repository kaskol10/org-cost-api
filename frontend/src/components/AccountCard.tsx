import { useMemo, useState } from "react";
import { fetchServiceDetail } from "../api";
import type { AccountDashboard, ServiceDetail } from "../types";
import { formatCountSize, formatCurrency, formatServiceName } from "../utils/format";
import DailyCostChart from "./DailyCostChart";
import ServiceDetailPanel from "./ServiceDetailPanel";
import StorageSummaryStrip from "./StorageSummaryStrip";
import UsageCategoryPanel from "./UsageCategoryPanel";
import UsageTable from "./UsageTable";
import UsageTypeTrends from "./UsageTypeTrends";

interface Props {
  account: AccountDashboard;
}

export default function AccountCard({ account }: Props) {
  const [serviceOpen, setServiceOpen] = useState<string | null>(null);
  const [serviceLoading, setServiceLoading] = useState(false);
  const [serviceError, setServiceError] = useState<string | null>(null);
  const [serviceDetail, setServiceDetail] = useState<ServiceDetail | null>(null);

  const serviceRows = useMemo(() => {
    const costs = account.costs;
    if (!costs) return [];
    return (costs.by_service ?? []).map((s) => ({
      raw: s.service,
      label: formatServiceName(s.service),
      sublabel: formatServiceName(s.service) !== s.service ? s.service : undefined,
      amount: s.amount,
      percent: s.percent,
    }));
  }, [account.costs?.by_service]);

  async function toggleServiceDetail(service: string) {
    const next = serviceOpen === service ? null : service;
    setServiceOpen(next);
    setServiceError(null);
    setServiceDetail(null);
    if (!next || !account.costs) return;

    setServiceLoading(true);
    try {
      const detail = await fetchServiceDetail({
        account_id: account.account_id,
        service,
        start: account.costs.start,
        end: account.costs.end,
      });
      setServiceDetail(detail);
    } catch (e) {
      setServiceError(e instanceof Error ? e.message : "Failed to load service detail");
    } finally {
      setServiceLoading(false);
    }
  }

  if (account.error) {
    return (
      <article className="account-card" id={`account-${account.account_id}`}>
        <header>
          <div>
            <h3>{account.account_name}</h3>
            <div className="meta mono">Account {account.account_id}</div>
          </div>
        </header>
        <div className="error" style={{ margin: "1rem" }}>
          {account.error}
        </div>
      </article>
    );
  }

  const { costs, snapshots, volumes } = account;
  if (!costs || !snapshots) {
    return null;
  }

  const snapshotAPIRows = (costs.by_api_operation ?? []).filter((op) =>
    /snapshot|create/i.test(op.operation)
  );
  const snapshotCostRows = (costs.snapshot_usage ?? []).map((s) => ({
    label: s.usage_type,
    amount: s.amount,
  }));

  const storageMeta = [
    volumes
      ? formatCountSize(volumes.count, volumes.total_gib, volumes.count === 1 ? "disk" : "disks")
      : null,
    formatCountSize(
      snapshots.count,
      snapshots.total_size_gib,
      snapshots.count === 1 ? "snapshot" : "snapshots"
    ),
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <article className="account-card" id={`account-${account.account_id}`}>
      <header>
        <div>
          <h3>{account.account_name}</h3>
          <div className="meta mono">
            Account {account.account_id} · {snapshots.region}
          </div>
        </div>
        <div className="meta">
          Org: <strong>{formatCurrency(costs.all_total)}</strong>
          {" · "}
          EC2-Other: <strong>{formatCurrency(costs.total)}</strong>
          {" · "}
          <span title="Live EC2 inventory">{storageMeta}</span>
        </div>
      </header>
      <div className="body">
        <div className="body-full">
          <StorageSummaryStrip
            volumes={volumes}
            snapshots={snapshots}
            volumesNote="Volume count/size is live inventory, not per-day history (deleted volumes are not included)."
            snapshotsNote="Snapshots: owner=self in this region."
          />
        </div>

        <div className="body-full">
          <div className="kpi-grid" style={{ marginBottom: "1rem" }}>
            <div className="kpi">
              <label>All services (usage)</label>
              <div className="value">{formatCurrency(costs.all_total)}</div>
            </div>
            <div className="kpi">
              <label>EC2-Other</label>
              <div className="value">{formatCurrency(costs.total)}</div>
            </div>
            <div className="kpi">
              <label>Other services</label>
              <div className="value">{formatCurrency(costs.other_services_total)}</div>
              <div className="kpi-sub">S3, RDS, data transfer, …</div>
            </div>
          </div>
        </div>

        <div>
          <div>
            <h2>Top AWS services (usage-only)</h2>
            <p className="category-desc" style={{ marginTop: "0.25rem" }}>
              Click a row to drill down.
            </p>
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Service</th>
                    <th className="amount">Share</th>
                    <th className="amount">Cost</th>
                  </tr>
                </thead>
                <tbody>
                  {serviceRows.map((r) => (
                    <tr
                      key={r.raw}
                      className={serviceOpen === r.raw ? "row-active" : undefined}
                      style={{ cursor: "pointer" }}
                      onClick={() => toggleServiceDetail(r.raw)}
                      title="Show service detail"
                    >
                      <td className="mono">
                        {r.sublabel ? (
                          <>
                            <span style={{ color: "var(--muted)" }}>{r.sublabel}</span>
                            {" · "}
                            {r.label}
                          </>
                        ) : (
                          r.label
                        )}
                      </td>
                      <td className="amount">
                        {r.percent != null ? `${r.percent.toFixed(1)}%` : "—"}
                      </td>
                      <td className="amount">{formatCurrency(r.amount)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {serviceOpen && (
            <div style={{ marginTop: "1rem" }}>
              <div style={{ display: "flex", justifyContent: "space-between", gap: "1rem", alignItems: "baseline" }}>
                <div className="category-desc" style={{ margin: "0 0 0.35rem" }}>
                  Selected: <span className="mono">{serviceOpen}</span>
                </div>
                <button
                  className="btn"
                  type="button"
                  onClick={() => setServiceOpen(null)}
                  style={{ padding: "0.35rem 0.6rem", fontSize: "0.85rem" }}
                >
                  Close
                </button>
              </div>
              {serviceLoading && <p className="loading">Loading service detail…</p>}
              {serviceError && <div className="error">{serviceError}</div>}
              {serviceDetail && <ServiceDetailPanel detail={serviceDetail} />}
            </div>
          )}
        </div>

        <div className="body-full">
          <DailyCostChart
            data={costs.daily}
            title="Daily EC2-Other cost"
          />
        </div>

        <div className="body-full">
          <UsageCategoryPanel
            categories={costs.usage_categories ?? []}
            total={costs.total}
          />
        </div>

        <div>
          <UsageTable
            title="All usage types"
            showPercent
            rows={(costs.by_usage_type ?? []).map((u) => ({
              label: u.short_name || u.usage_type,
              sublabel: u.region,
              amount: u.amount,
              percent: u.percent,
            }))}
          />
        </div>

        <div className="body-full">
          <UsageTypeTrends series={costs.top_usage_daily ?? []} />
        </div>

        <div>
          <UsageTable
            title="Snapshot storage charges"
            rows={snapshotCostRows}
            labelHeader="Usage type"
          />
        </div>

        <div>
          <UsageTable
            title="Snapshot-related API operations"
            rows={snapshotAPIRows.map((op) => ({
              label: op.operation,
              amount: op.amount,
            }))}
            labelHeader="API operation"
          />
          {snapshots.recent_creates && snapshots.recent_creates.length > 0 && (
            <div style={{ marginTop: "1rem" }}>
              <h2>Recent snapshots</h2>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Snapshot</th>
                      <th>Size</th>
                      <th>Created</th>
                    </tr>
                  </thead>
                  <tbody>
                    {snapshots.recent_creates.map((s) => (
                      <tr key={s.snapshot_id}>
                        <td className="mono">{s.snapshot_id}</td>
                        <td>{s.size_gib} GiB</td>
                        <td>
                          {new Date(s.start_time).toLocaleString()}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      </div>
    </article>
  );
}
