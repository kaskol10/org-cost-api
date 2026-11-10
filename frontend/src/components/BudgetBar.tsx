import type { BudgetStatus, ForecastResult } from "../types";
import { formatCurrency } from "../utils/format";

interface Props {
  forecast?: ForecastResult | null;
  budgets?: BudgetStatus[] | null;
}

/** Projected end-of-month spend vs the org budget (or the first budget). */
export default function BudgetBar({ forecast, budgets }: Props) {
  const primary = (budgets ?? []).find((b) => !b.account) ?? budgets?.[0];
  if (!forecast || !primary) return null;
  const pct = Math.round(primary.percent_of_budget);
  const statusClass =
    primary.status === "over" ? "budget-over" : primary.status === "warning" ? "budget-warning" : "budget-ok";

  return (
    <div className={`cost-context-budget ${statusClass}`}>
      <div className="cost-context-budget-labels">
        <span>
          Projected end of month <strong>{formatCurrency(forecast.projected_usd)}</strong>
        </span>
        <span>
          {primary.name} budget <strong>{formatCurrency(primary.monthly_usd)}</strong> · {pct}%
        </span>
      </div>
      <div className="cost-context-budget-track" role="img" aria-label={`${pct}% of budget projected`}>
        <div className="cost-context-budget-fill" style={{ width: `${Math.min(100, primary.percent_of_budget)}%` }} />
      </div>
      {primary.status !== "ok" && (
        <p className="cost-context-budget-note">
          {primary.status === "over"
            ? `Projected ${formatCurrency(primary.over_by_usd ?? 0)} over budget.`
            : "On track to breach budget this month."}
        </p>
      )}
    </div>
  );
}
