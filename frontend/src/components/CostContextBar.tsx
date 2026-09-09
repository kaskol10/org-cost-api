import type { PeriodPreset } from "../api";
import type { TrendsResponse } from "../types";
import {
  formatCurrency,
  formatSpendChangePill,
  spendDirection,
} from "../utils/format";
import { periodLabel, priorCompareLabel } from "../utils/periodLabels";

interface Props {
  orgTotal: number | null;
  accountCount?: number;
  dateRange?: string;
  trends: TrendsResponse | null;
  trendsLoading?: boolean;
  period: PeriodPreset;
  onPeriodChange: (period: PeriodPreset) => void;
  onRefresh: () => void;
  loading?: boolean;
  demoMode?: boolean;
}

export default function CostContextBar({
  orgTotal,
  accountCount,
  dateRange,
  trends,
  trendsLoading,
  period,
  onPeriodChange,
  onRefresh,
  loading,
  demoMode,
}: Props) {
  const orgChange = trends?.org_total;
  const hasPrior = orgChange && orgChange.prior_usd > 0;
  const compareLabel = priorCompareLabel(period);
  const activePeriodLabel = periodLabel(period, trends?.current_period?.days);
  const spendDir = orgChange
    ? spendDirection(orgChange.change_usd, orgChange.change_percent)
    : "flat";
  const changePillClass =
    spendDir === "up" ? "up" : spendDir === "down" ? "down" : "flat";

  return (
    <section className="cost-context-bar" aria-label="Organization cost context">
      <div className="cost-context-main">
        <div className="cost-context-total-block">
          <p className="cost-context-eyebrow">
            Org usage spend · {activePeriodLabel}
            {demoMode && <span className="cost-context-demo-chip">Demo data</span>}
          </p>
          <div className="cost-context-total-row">
            {orgTotal == null ? (
              <p className="cost-context-total muted">{loading ? "Loading…" : "—"}</p>
            ) : (
              <p className={`cost-context-total ${loading ? "is-updating" : ""}`}>
                {formatCurrency(orgTotal)}
              </p>
            )}
            {trendsLoading && (
              <span className="summary-change-pill loading">Comparing…</span>
            )}
            {!trendsLoading && hasPrior && orgChange && (
              <span className={`summary-change-pill ${changePillClass}`}>
                {formatSpendChangePill(
                  orgChange.change_usd,
                  orgChange.change_percent,
                  compareLabel
                )}
              </span>
            )}
          </div>
          <p className="cost-context-meta">
            {dateRange || "Date range pending"}
            {accountCount != null && (
              <>
                <span className="summary-dot">·</span>
                {accountCount} accounts
              </>
            )}
          </p>
          {loading && orgTotal != null && (
            <p className="cost-context-updating" role="status">
              Updating for {activePeriodLabel.toLowerCase()}…
            </p>
          )}
        </div>
      </div>

      <div className="cost-context-actions">
        <div className="period-toggle" role="group" aria-label="Cost period">
          <button
            type="button"
            className={period === "30d" ? "period-toggle-btn active" : "period-toggle-btn"}
            aria-pressed={period === "30d"}
            onClick={() => onPeriodChange("30d")}
            disabled={loading}
          >
            Last 30 days
          </button>
          <button
            type="button"
            className={period === "mtd" ? "period-toggle-btn active" : "period-toggle-btn"}
            aria-pressed={period === "mtd"}
            onClick={() => onPeriodChange("mtd")}
            disabled={loading}
          >
            Month to date
          </button>
        </div>
        <button
          className="btn"
          type="button"
          onClick={onRefresh}
          disabled={loading}
          title="Fetches fresh totals from AWS Cost Explorer when cache is cold"
        >
          {loading ? "Refreshing…" : "Refresh from AWS"}
        </button>
      </div>
    </section>
  );
}
