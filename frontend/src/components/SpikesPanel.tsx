import type { AccountDashboard, Spike } from "../types";
import { formatCurrency, formatSignedCurrency, formatSignedPercent } from "../utils/format";

interface Props {
  spikes: Spike[];
  accounts: AccountDashboard[];
  onAskAbout?: (question: string) => void;
}

function spikeAskQuestion(s: Spike, names: string[]): string {
  const where = names.length ? ` in ${names.join(" and ")}` : "";
  const pct = Math.round(Math.abs(s.deviation_pct));
  const direction = s.direction === "up" ? `rose ${pct}%` : `dropped ${pct}%`;
  return `Spend ${direction} on ${s.date} (${formatCurrency(s.amount_usd)} vs ~${formatCurrency(
    s.baseline_usd
  )} baseline)${where}. What likely caused it?`;
}

export default function SpikesPanel({ spikes, accounts, onAskAbout }: Props) {
  if (!spikes.length) return null;

  const nameById = new Map(accounts.map((a) => [a.account_id, a.account_name]));
  const namesFor = (ids?: string[]) =>
    (ids ?? []).map((id) => nameById.get(id) ?? id);

  return (
    <section className="panel spikes-panel" aria-label="Daily spend spikes">
      <div className="spikes-head">
        <h2>Daily spikes</h2>
        <p className="spikes-sub">
          Days where org spend deviated from the trailing 14-day baseline
          (≥50% and ≥$50).
        </p>
      </div>
      <ul className="spikes-list">
        {spikes.map((s) => {
          const up = s.direction === "up";
          return (
            <li
              key={s.date}
              className={`spike-item ${up ? "spike-up" : "spike-down"}`}
            >
              <div className="spike-row">
                <span className="spike-date">{s.date}</span>
                <span className={`spike-delta ${up ? "up" : "down"}`}>
                  {formatSignedCurrency(s.amount_usd - s.baseline_usd)} (
                  {formatSignedPercent(s.deviation_pct)})
                </span>
              </div>
              <div className="spike-meta">
                {formatCurrency(s.baseline_usd)} → {formatCurrency(s.amount_usd)}
                {s.incomplete && " · day still settling (CE lag)"}
              </div>
              {namesFor(s.account_ids).length > 0 && (
                <div className="spike-accounts">
                  {namesFor(s.account_ids).join(" · ")}
                </div>
              )}
              {onAskAbout && (
                <button
                  type="button"
                  className="btn btn-ghost spike-ask-btn"
                  onClick={() => onAskAbout(spikeAskQuestion(s, namesFor(s.account_ids)))}
                >
                  Ask about this →
                </button>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
