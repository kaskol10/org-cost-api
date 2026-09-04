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
import { formatCurrency, formatGiB, formatPercent, formatServiceName } from "../utils/format";
import { fetchServiceTagDelta, fetchServiceTagTotals } from "../api";

interface Props {
  totals: ConsolidatedTotals;
  topServices: OrgServiceDriver[];
  accounts: AccountDashboard[];
  orgDaily: DailyCost[];
  dateRange: string;
  trends?: TrendsResponse | null;
  trendsLoading?: boolean;
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
  return (
    <li
      className={`summary-trend-item ${active ? "summary-trend-active" : ""} ${
        onClick ? "summary-trend-clickable" : ""
      }`}
      style={onClick ? { cursor: "pointer" } : undefined}
      onClick={onClick}
      role={onClick ? "button" : undefined}
      aria-label={onClick ? `Explain change for ${name}` : undefined}
    >
      <div className="summary-bar-row">
        <span className="summary-bar-name">{name}</span>
        <span className={`summary-trend-delta ${positive ? "up" : "down"}`}>
          {formatCurrency(item.change_usd)} ({formatPercent(item.change_percent)})
        </span>
      </div>
      <div className="summary-trend-meta">
        {formatCurrency(item.prior_usd)} → {formatCurrency(item.current_usd)}
      </div>
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
}: Props) {
  const top = topServices.slice(0, 5);
  const unattachedGiB = totals.volume_available_gib ?? 0;
  const estWasteMonthly = unattachedGiB * GP3_USD_PER_GIB;

  const wasteAccounts = accounts
    .filter((a) => (a.volumes?.available_count ?? 0) > 0)
    .map((a) => ({
      name: a.account_name,
      count: a.volumes!.available_count!,
      gib: a.volumes!.available_gib ?? 0,
    }))
    .sort((a, b) => b.gib - a.gib)
    .slice(0, 4);

  const ec2OtherShare =
    totals.org_total > 0 ? (totals.ec2_other_cost / totals.org_total) * 100 : 0;

  const orgChange = trends?.org_total;
  const hasPrior = orgChange && orgChange.prior_usd > 0;
  const changeUp = (orgChange?.change_percent ?? 0) > 0;

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

  async function loadExplain(service: string) {
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
            {formatCurrency(d.change_usd)} ({formatPercent(d.change_percent)})
          </span>
        </div>
        <div className="summary-trend-meta">
          {formatCurrency(d.prior_usd)} → {formatCurrency(d.current_usd)}
        </div>
      </li>
    );
  }

  return (
    <section className="summary-dashboard" aria-label="Organization summary">
      <div className="summary-hero">
        <div className="summary-hero-main">
          <p className="summary-eyebrow">Organization usage spend</p>
          <div className="summary-total-row">
            <p className="summary-total">{formatCurrency(totals.org_total)}</p>
            {trendsLoading && (
              <span className="summary-change-pill loading">Comparing…</span>
            )}
            {!trendsLoading && hasPrior && orgChange && (
              <span className={`summary-change-pill ${changeUp ? "up" : "down"}`}>
                {formatPercent(orgChange.change_percent)} vs prior
              </span>
            )}
          </div>
          <p className="summary-meta">
            {dateRange}
            <span className="summary-dot">·</span>
            {totals.account_count} accounts
            <span className="summary-dot">·</span>
            EC2-Other {formatCurrency(totals.ec2_other_cost)} ({ec2OtherShare.toFixed(0)}%)
          </p>
          {!trendsLoading && hasPrior && orgChange && trends && (
            <p className="summary-prior">
              Prior period ({formatPeriodRange(trends.prior_period)}):{" "}
              <strong>{formatCurrency(orgChange.prior_usd)}</strong>
              <span className="summary-dot">·</span>
              {formatCurrency(orgChange.change_usd)} change
            </p>
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
              {top.map((s) => (
                <li key={s.service}>
                  <div className="summary-bar-row">
                    <span className="summary-bar-name">{formatServiceName(s.service)}</span>
                    <span className="summary-bar-value">{formatCurrency(s.amount)}</span>
                  </div>
                  <div className="summary-bar-track">
                    <div
                      className="summary-bar-fill"
                      style={{ width: `${Math.min(100, s.percent ?? 0)}%` }}
                    />
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>

        <div className="summary-card">
          <h2 className="summary-card-title">vs prior period</h2>
          {trendsLoading && <p className="summary-empty">Loading comparison…</p>}
          {!trendsLoading && !trends && (
            <p className="summary-empty">Trend comparison unavailable</p>
          )}
          {!trendsLoading && trends && (
            <>
              {(trends.top_increases?.length ?? 0) > 0 && (
                <>
                  <p className="summary-trend-label">Increases</p>
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
                  <p className="summary-trend-label">Decreases</p>
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

              {explainService && (
                <div className="summary-explain">
                  <div className="summary-explain-head">
                    <div className="summary-explain-title">
                      Why {explain?.display_name ?? formatServiceName(explainService)}
                      {explainLoading && <span className="summary-muted"> · fetching…</span>}
                    </div>
                    {explain && (
                      <span
                        className={`summary-change-pill ${
                          (explain.delta_usd ?? 0) >= 0 ? "up" : "down"
                        }`}
                        title="Change in total service spend"
                      >
                        {formatPercent(explain.delta_percent)} vs prior
                      </span>
                    )}
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
                                  {formatCurrency(a.delta_usd)}
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
