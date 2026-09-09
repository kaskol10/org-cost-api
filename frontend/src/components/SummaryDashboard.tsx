import { useEffect, useMemo, useRef, useState } from "react";
import type {
  AccountDashboard,
  ConsolidatedTotals,
  DailyCost,
  OrgServiceDriver,
  ServiceTagDeltaResponse,
  ServiceTagTotalsResponse,
  ServiceTrend,
  TrendsResponse,
} from "../types";
import type { PeriodPreset } from "../api";
import {
  formatCurrency,
  formatGiB,
  formatPercent,
  formatServiceName,
  formatServiceSpendDelta,
  formatSignedCurrency,
  formatSignedPercent,
} from "../utils/format";
import { periodLabel, priorCompareLabel } from "../utils/periodLabels";
import { fetchServiceTagDelta, fetchServiceTagTotals } from "../api";

interface Props {
  totals: ConsolidatedTotals;
  topServices: OrgServiceDriver[];
  accounts: AccountDashboard[];
  orgDaily: DailyCost[];
  dateRange: string;
  trends?: TrendsResponse | null;
  trendsLoading?: boolean;
  period: PeriodPreset;
}

const GP3_USD_PER_GIB = 0.08;

function formatPeriodRange(p: { start: string; end: string }) {
  const fmt = (d: string) =>
    new Date(d + "T00:00:00").toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
    });
  return `${fmt(p.start)} – ${fmt(p.end)}`;
}

function MiniSparkline({ data }: { data: DailyCost[] }) {
  const points = data.slice(-21);
  if (points.length < 2) return null;

  const w = 200;
  const h = 48;
  const pad = 2;
  const max = Math.max(...points.map((p) => p.amount), 1);
  const min = Math.min(...points.map((p) => p.amount));
  const span = max - min || max;

  const coords = points.map((p, i) => {
    const x = pad + (i / (points.length - 1)) * (w - pad * 2);
    const y = h - pad - ((p.amount - min) / span) * (h - pad * 2);
    return `${x},${y}`;
  });

  return (
    <svg
      className="summary-sparkline"
      viewBox={`0 0 ${w} ${h}`}
      width={w}
      height={h}
      aria-hidden
    >
      <polyline
        fill="none"
        stroke="var(--accent)"
        strokeWidth="2"
        strokeLinejoin="round"
        strokeLinecap="round"
        points={coords.join(" ")}
      />
    </svg>
  );
}

function TrendRow({
  item,
  positive,
  active,
  onClick,
}: {
  item: ServiceTrend;
  positive: boolean;
  active?: boolean;
  onClick?: () => void;
}) {
  const name = item.display_name || formatServiceName(item.service);
  const body = (
    <>
      <div className="summary-bar-row">
        <span className="summary-bar-name">{name}</span>
        <span className={`summary-trend-delta ${positive ? "up" : "down"}`}>
          {formatServiceSpendDelta(item.change_usd, item.change_percent)}
        </span>
      </div>
      <div className="summary-trend-meta">
        {formatCurrency(item.prior_usd)} → {formatCurrency(item.current_usd)}
      </div>
    </>
  );

  return (
    <li className={`summary-trend-item ${active ? "summary-trend-active" : ""}`}>
      {onClick ? (
        <button
          type="button"
          className="summary-trend-btn"
          onClick={onClick}
          aria-pressed={active}
          aria-label={`Explain change for ${name}`}
        >
          {body}
        </button>
      ) : (
        body
      )}
    </li>
  );
}

export default function SummaryDashboard({
  totals,
  topServices,
  accounts,
  orgDaily,
  dateRange,
  trends,
  trendsLoading,
  period,
}: Props) {
  const top = topServices.slice(0, 5);
  const unattachedGiB = totals.volume_available_gib ?? 0;
  const estWasteMonthly = unattachedGiB * GP3_USD_PER_GIB;
  const lookbackDays = trends?.current_period?.days;
  const activePeriodLabel = periodLabel(period, lookbackDays);
  const compareLabel = priorCompareLabel(period);

  const wasteAccounts = accounts
    .filter((a) => (a.volumes?.available_count ?? 0) > 0)
    .map((a) => ({
      name: a.account_name,
      count: a.volumes!.available_count!,
      gib: a.volumes!.available_gib ?? 0,
    }))
    .sort((a, b) => b.gib - a.gib)
    .slice(0, 4);

  const orgChange = trends?.org_total;
  const hasPrior = orgChange && orgChange.prior_usd > 0;

  const [explainLoading, setExplainLoading] = useState(false);
  const [explainError, setExplainError] = useState<string | null>(null);
  const [totalsNote, setTotalsNote] = useState<string | null>(null);
  const [explainService, setExplainService] = useState<string | null>(null);
  const [explain, setExplain] = useState<ServiceTagDeltaResponse | null>(null);
  const [tagTotals, setTagTotals] = useState<ServiceTagTotalsResponse | null>(null);
  const [activeAccountId, setActiveAccountId] = useState<string | null>(null);
  const explainAbortRef = useRef<AbortController | null>(null);
  const explainGenRef = useRef(0);

  useEffect(() => {
    return () => {
      explainAbortRef.current?.abort();
    };
  }, []);

  function clearExplain() {
    explainAbortRef.current?.abort();
    explainGenRef.current += 1;
    setExplainLoading(false);
    setExplainError(null);
    setTotalsNote(null);
    setExplainService(null);
    setExplain(null);
    setTagTotals(null);
    setActiveAccountId(null);
  }

  async function loadExplain(service: string) {
    if (explainService === service) {
      clearExplain();
      return;
    }

    explainAbortRef.current?.abort();
    const ac = new AbortController();
    explainAbortRef.current = ac;
    const gen = ++explainGenRef.current;

    setExplainLoading(true);
    setExplainError(null);
    setTotalsNote(null);
    setExplainService(service);
    setExplain(null);
    setTagTotals(null);
    try {
      const start = trends?.current_period?.start;
      const end = trends?.current_period?.end;
      if (!start || !end) {
        throw new Error("Trend period unavailable");
      }
      const [deltaSettled, totalsSettled] = await Promise.allSettled([
        fetchServiceTagDelta({
          service,
          start,
          end,
          top_accounts: 5,
          tag_key: "Name",
          signal: ac.signal,
        }),
        fetchServiceTagTotals({
          service,
          start,
          end,
          tag_key: "Name",
          top_buckets: 10,
          signal: ac.signal,
        }),
      ]);
      if (gen !== explainGenRef.current || ac.signal.aborted) return;

      if (deltaSettled.status === "rejected") {
        const e = deltaSettled.reason;
        if (e instanceof DOMException && e.name === "AbortError") return;
        setExplainError(e instanceof Error ? e.message : "Failed to load explanation");
        setExplain(null);
        setTagTotals(null);
        return;
      }

      setExplain(deltaSettled.value);
      setActiveAccountId(deltaSettled.value.accounts[0]?.account_id ?? null);

      if (totalsSettled.status === "fulfilled") {
        setTagTotals(totalsSettled.value);
      } else {
        const e = totalsSettled.reason;
        if (!(e instanceof DOMException && e.name === "AbortError")) {
          setTagTotals(null);
          setTotalsNote("Current tag totals unavailable");
        }
      }
    } catch (e) {
      if (gen !== explainGenRef.current || ac.signal.aborted) return;
      if (e instanceof DOMException && e.name === "AbortError") return;
      setExplainError(e instanceof Error ? e.message : "Failed to load explanation");
      setExplain(null);
      setTagTotals(null);
    } finally {
      if (gen === explainGenRef.current && !ac.signal.aborted) {
        setExplainLoading(false);
      }
    }
  }

  const activeAccount = useMemo(
    () => explain?.accounts.find((a) => a.account_id === activeAccountId) ?? null,
    [explain, activeAccountId]
  );

  function TagDeltaRow({
    d,
    positive,
  }: {
    d: ServiceTagDeltaResponse["top_increases"][0];
    positive: boolean;
  }) {
    return (
      <li className="summary-trend-item">
        <div className="summary-bar-row">
          <span className="summary-bar-name mono">{d.key}</span>
          <span className={`summary-trend-delta ${positive ? "up" : "down"}`}>
            {formatServiceSpendDelta(d.change_usd, d.change_percent)}
          </span>
        </div>
        <div className="summary-trend-meta">
          {formatCurrency(d.prior_usd)} → {formatCurrency(d.current_usd)}
        </div>
      </li>
    );
  }

  const hasTrendMovers =
    (trends?.top_increases?.length ?? 0) > 0 || (trends?.top_decreases?.length ?? 0) > 0;

  const explainPickServices = useMemo(() => {
    if (!trends) return [];
    const seen = new Set<string>();
    const picks: ServiceTrend[] = [];
    for (const t of [
      ...(trends.top_increases ?? []).slice(0, 4),
      ...(trends.top_decreases ?? []).slice(0, 4),
    ]) {
      if (seen.has(t.service)) continue;
      seen.add(t.service);
      picks.push(t);
    }
    return picks;
  }, [trends]);

  return (
    <section className="summary-dashboard" aria-label="Organization summary">
      <div className="summary-hero summary-hero-compact">
        <div className="summary-hero-main">
          <p className="summary-eyebrow">Explore breakdown · {activePeriodLabel}</p>
          <p className="summary-meta">
            {dateRange}
            <span className="summary-dot">·</span>
            {totals.account_count} accounts
          </p>
          {!trendsLoading && hasPrior && orgChange && trends && (
            <p className="summary-prior">
              Prior ({formatPeriodRange(trends.prior_period)}):{" "}
              <strong>{formatCurrency(orgChange.prior_usd)}</strong>
            </p>
          )}
          {trendsLoading && (
            <p className="summary-prior">Loading comparison details…</p>
          )}
        </div>
        {orgDaily.length > 1 && (
          <div className="summary-sparkline-wrap">
            <span className="summary-sparkline-label">Daily trend</span>
            <MiniSparkline data={orgDaily} />
          </div>
        )}
      </div>

      <div className="summary-columns summary-columns-3">
        <div className="summary-card">
          <h2 className="summary-card-title">Top services</h2>
          {top.length === 0 ? (
            <p className="summary-empty">No service data</p>
          ) : (
            <ul className="summary-bar-list">
              {top.map((s) => {
                const active = explainService === s.service;
                const name = formatServiceName(s.service);
                return (
                  <li
                    key={s.service}
                    className={`summary-trend-item ${active ? "summary-trend-active" : ""}`}
                  >
                    <button
                      type="button"
                      className="summary-trend-btn"
                      onClick={() => loadExplain(s.service)}
                      aria-pressed={active}
                      aria-label={`Explain change for ${name}`}
                      disabled={!trends?.current_period?.start || !trends?.current_period?.end}
                    >
                      <div className="summary-bar-row">
                        <span className="summary-bar-name">{name}</span>
                        <span className="summary-bar-value">{formatCurrency(s.amount)}</span>
                      </div>
                      <div className="summary-bar-track">
                        <div
                          className="summary-bar-fill"
                          style={{ width: `${Math.min(100, s.percent ?? 0)}%` }}
                        />
                      </div>
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        <div className="summary-card">
          <h2 className="summary-card-title">{compareLabel}</h2>
          {trendsLoading && <p className="summary-empty">Loading comparison…</p>}
          {!trendsLoading && !trends && (
            <p className="summary-empty">Trend comparison unavailable</p>
          )}
          {!trendsLoading && trends && (
            <>
              {period === "mtd" && trends.prior_period?.start && (
                <p className="summary-trend-label">
                  Prior MTD {formatPeriodRange(trends.prior_period)}
                </p>
              )}
              {(trends.top_increases?.length ?? 0) > 0 && (
                <>
                  <p className="summary-trend-label">Higher spend</p>
                  <ul className="summary-trend-list">
                    {trends.top_increases.slice(0, 4).map((t) => (
                      <TrendRow
                        key={t.service}
                        item={t}
                        positive
                        active={explainService === t.service}
                        onClick={() => loadExplain(t.service)}
                      />
                    ))}
                  </ul>
                </>
              )}
              {(trends.top_decreases?.length ?? 0) > 0 && (
                <>
                  <p className="summary-trend-label">Lower spend</p>
                  <ul className="summary-trend-list">
                    {trends.top_decreases.slice(0, 4).map((t) => (
                      <TrendRow
                        key={t.service}
                        item={t}
                        positive={false}
                        active={explainService === t.service}
                        onClick={() => loadExplain(t.service)}
                      />
                    ))}
                  </ul>
                </>
              )}
              {!(trends.top_increases?.length || trends.top_decreases?.length) && (
                <p className="summary-empty">No significant changes</p>
              )}

              {hasTrendMovers && !explainService && (
                <div className="summary-explain-empty" role="status">
                  <p className="summary-explain-empty-copy">
                    Pick a service to see which accounts/tags drove the change.
                  </p>
                  <div className="summary-explain-picks">
                    {explainPickServices.map((t) => (
                      <button
                        key={t.service}
                        type="button"
                        className="summary-explain-pick"
                        onClick={() => loadExplain(t.service)}
                      >
                        {t.display_name || formatServiceName(t.service)}
                      </button>
                    ))}
                  </div>
                </div>
              )}

              {explainService && (
                <div className="summary-explain">
                  <div className="summary-explain-head">
                    <div className="summary-explain-title">
                      Why {explain?.display_name ?? formatServiceName(explainService)}
                      {explainLoading && <span className="summary-muted"> · fetching…</span>}
                    </div>
                    <div className="summary-explain-head-actions">
                      {explain && (
                        <span
                          className={`summary-change-pill ${
                            (explain.delta_usd ?? 0) >= 0 ? "up" : "down"
                          }`}
                          title="Change in total service spend"
                        >
                          {formatSignedCurrency(explain.delta_usd ?? 0)} ·{" "}
                          {formatSignedPercent(explain.delta_percent)} vs prior
                        </span>
                      )}
                      <button
                        type="button"
                        className="summary-explain-clear"
                        onClick={clearExplain}
                        aria-label="Clear service explanation"
                      >
                        Clear
                      </button>
                    </div>
                  </div>

                  {explainError && <div className="error">{explainError}</div>}
                  {explainLoading && <p className="summary-empty">Loading tag drivers…</p>}
                  {totalsNote && !explainError && (
                    <p className="category-desc" style={{ marginTop: "0.35rem" }}>
                      {totalsNote}
                    </p>
                  )}

                  {explain && (
                    <>
                      {explain.top_account_note && (
                        <p className="category-desc" style={{ marginTop: "0.35rem" }}>
                          {explain.top_account_note}
                        </p>
                      )}

                      <div className="summary-explain-grid">
                        <div>
                          <p className="summary-trend-label">Top bucket increases ({explain.tag_key})</p>
                          <ul className="summary-trend-list">
                            {(explain.top_increases ?? []).map((d) => (
                              <TagDeltaRow key={d.key} d={d} positive />
                            ))}
                            {(explain.top_increases ?? []).length === 0 && (
                              <li className="summary-empty">None</li>
                            )}
                          </ul>
                        </div>
                        <div>
                          <p className="summary-trend-label">Top bucket decreases ({explain.tag_key})</p>
                          <ul className="summary-trend-list">
                            {(explain.top_decreases ?? []).map((d) => (
                              <TagDeltaRow key={d.key} d={d} positive={false} />
                            ))}
                            {(explain.top_decreases ?? []).length === 0 && (
                              <li className="summary-empty">None</li>
                            )}
                          </ul>
                        </div>
                      </div>

                      {tagTotals && (tagTotals.buckets?.length ?? 0) > 0 && (
                        <div style={{ marginTop: "0.8rem" }}>
                          <p className="summary-trend-label">
                            Current top buckets ({tagTotals.tag_key})
                          </p>
                          <ul className="summary-trend-list">
                            {tagTotals.buckets.slice(0, 10).map((b) => (
                              <li key={b.key} className="summary-trend-item">
                                <div className="summary-bar-row">
                                  <span className="summary-bar-name mono">{b.key}</span>
                                  <span className="summary-trend-delta">
                                    {formatCurrency(b.current_usd)}
                                    {b.current_share_pct != null &&
                                      ` (${formatPercent(b.current_share_pct)})`}
                                  </span>
                                </div>
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}

                      {explain.accounts.length > 0 && (
                        <div className="summary-explain-accounts">
                          <p className="summary-trend-label" style={{ marginTop: "0.8rem" }}>
                            Top accounts contributing
                          </p>
                          <ul className="summary-account-list">
                            {explain.accounts.map((a) => (
                              <li
                                key={a.account_id}
                                className={`summary-account-item ${
                                  activeAccountId === a.account_id ? "active" : ""
                                }`}
                                onClick={() => setActiveAccountId(a.account_id)}
                                role="button"
                                aria-label={`Show tag drivers for ${a.account_name}`}
                              >
                                <span className="mono">{a.account_name}</span>
                                <span
                                  className={`summary-trend-delta ${
                                    a.delta_usd >= 0 ? "up" : "down"
                                  }`}
                                >
                                  {formatSignedCurrency(a.delta_usd)}
                                </span>
                              </li>
                            ))}
                          </ul>

                          {activeAccount && (
                            <div className="summary-account-detail">
                              <p className="summary-trend-label" style={{ marginTop: "0.6rem" }}>
                                {activeAccount.account_name} bucket drivers
                              </p>
                              <div className="summary-explain-grid">
                                <div>
                                  <ul className="summary-trend-list">
                                    {(activeAccount.top_increases ?? []).map((d) => (
                                      <TagDeltaRow key={d.key} d={d} positive />
                                    ))}
                                    {(activeAccount.top_increases ?? []).length === 0 && (
                                      <li className="summary-empty">None</li>
                                    )}
                                  </ul>
                                </div>
                                <div>
                                  <ul className="summary-trend-list">
                                    {(activeAccount.top_decreases ?? []).map((d) => (
                                      <TagDeltaRow key={d.key} d={d} positive={false} />
                                    ))}
                                    {(activeAccount.top_decreases ?? []).length === 0 && (
                                      <li className="summary-empty">None</li>
                                    )}
                                  </ul>
                                </div>
                              </div>
                            </div>
                          )}
                        </div>
                      )}
                    </>
                  )}
                </div>
              )}
            </>
          )}
        </div>

        <div className="summary-card">
          <h2 className="summary-card-title">Waste & storage</h2>
          <dl className="summary-stats">
            <div className="summary-stat">
              <dt>Unattached EBS</dt>
              <dd>
                {totals.volume_available_count > 0 ? (
                  <>
                    <strong>{totals.volume_available_count}</strong> disks ·{" "}
                    {formatGiB(unattachedGiB)}
                    <span className="summary-stat-sub">
                      ~{formatCurrency(estWasteMonthly)}/mo est.
                    </span>
                  </>
                ) : (
                  <span className="summary-ok">None detected</span>
                )}
              </dd>
            </div>
            <div className="summary-stat">
              <dt>EBS provisioned</dt>
              <dd>
                {totals.volume_count.toLocaleString()} disks ·{" "}
                {formatGiB(totals.volume_size_gib)}
              </dd>
            </div>
            <div className="summary-stat">
              <dt>Snapshots</dt>
              <dd>
                {totals.snapshot_count.toLocaleString()} ·{" "}
                {formatGiB(totals.snapshot_size_gib)}
              </dd>
            </div>
          </dl>
          {wasteAccounts.length > 0 && (
            <ul className="summary-waste-accounts">
              {wasteAccounts.map((a) => (
                <li key={a.name}>
                  <span className="mono">{a.name}</span>
                  <span>
                    {a.count} · {formatGiB(a.gib)}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </section>
  );
}
