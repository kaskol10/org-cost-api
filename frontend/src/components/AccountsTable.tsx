import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import type { AccountDashboard, AccountTrend } from "../types";
import { buildAccountRows, sortAccountRows, type SortDir, type SortKey } from "../utils/accountsTable";
import { formatCurrency, formatSignedCurrency, formatSignedPercent } from "../utils/format";
import AccountCard from "./AccountCard";

interface Props {
  accounts: AccountDashboard[];
  accountTrends?: AccountTrend[];
  orgTotal: number;
  compareLabel: string;
  selectedId: string | null;
  onSelect: (accountId: string | null) => void;
}

const COLUMNS: { key: SortKey; label: string; numeric: boolean; title?: string }[] = [
  { key: "name", label: "Account", numeric: false },
  { key: "spend", label: "Spend", numeric: true },
  { key: "change", label: "Change", numeric: true },
  { key: "changePct", label: "Change %", numeric: true },
  { key: "share", label: "Share", numeric: true },
  { key: "waste", label: "Unattached EBS", numeric: true, title: "Monthly storage cost of unattached EBS volumes, priced by volume type" },
];

/** Sortable accounts table; a row expands to the full account card. */
export default function AccountsTable({ accounts, accountTrends, orgTotal, compareLabel, selectedId, onSelect }: Props) {
  const [sortKey, setSortKey] = useState<SortKey>("change");
  const [sortDir, setSortDir] = useState<SortDir>("desc");
  const selectedRef = useRef<HTMLTableRowElement>(null);

  const rows = useMemo(
    () => sortAccountRows(buildAccountRows(accounts, accountTrends, orgTotal), sortKey, sortDir),
    [accounts, accountTrends, orgTotal, sortKey, sortDir]
  );
  const accountById = useMemo(() => new Map(accounts.map((a) => [a.account_id, a])), [accounts]);

  useEffect(() => {
    if (selectedId) selectedRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
  }, [selectedId]);

  function sortBy(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir(key === "name" ? "asc" : "desc");
    }
  }

  return (
    <section className="panel accounts-table-panel" aria-labelledby="accounts-table-title">
      <div className="accounts-table-head">
        <h2 id="accounts-table-title">Accounts</h2>
        <p className="accounts-table-sub">Change is {compareLabel}. Click an account for services, EC2-Other usage and storage.</p>
      </div>
      <div className="accounts-table-scroll">
        <table className="accounts-table">
          <thead>
            <tr>
              {COLUMNS.map((c) => (
                <th
                  key={c.key}
                  scope="col"
                  className={c.numeric ? "num" : undefined}
                  aria-sort={sortKey === c.key ? (sortDir === "asc" ? "ascending" : "descending") : "none"}
                >
                  <button type="button" className="th-sort" onClick={() => sortBy(c.key)} title={c.title}>
                    {c.label}
                    <span className="th-sort-icon" aria-hidden="true">
                      {sortKey === c.key ? (sortDir === "asc" ? "▲" : "▼") : ""}
                    </span>
                  </button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => {
              const open = r.id === selectedId;
              const acct = accountById.get(r.id);
              return (
                <Fragment key={r.id}>
                  <tr
                    ref={open ? selectedRef : undefined}
                    id={`account-${r.id}`}
                    className={`accounts-row${open ? " is-open" : ""}${r.error ? " has-error" : ""}`}
                  >
                    <th scope="row">
                      <button
                        type="button"
                        className="accounts-row-toggle"
                        aria-expanded={open}
                        onClick={() => onSelect(open ? null : r.id)}
                      >
                        <span className="accounts-row-caret" aria-hidden="true">{open ? "▾" : "▸"}</span>
                        {r.name}
                      </button>
                      {r.error && <span className="accounts-row-error">{r.error}</span>}
                    </th>
                    <td className="num">{r.error ? "—" : formatCurrency(r.spend)}</td>
                    <td className={`num ${r.change && r.change > 0 ? "delta-up" : r.change && r.change < 0 ? "delta-down" : ""}`}>
                      {r.change === null ? "—" : formatSignedCurrency(r.change)}
                    </td>
                    <td className="num">{r.changePct === null ? (r.change !== null ? "new" : "—") : formatSignedPercent(r.changePct)}</td>
                    <td className="num">{r.error ? "—" : `${r.share.toFixed(0)}%`}</td>
                    <td className="num">{r.wasteMonthly > 0 ? `${formatCurrency(r.wasteMonthly)}/mo` : "—"}</td>
                  </tr>
                  {open && acct && (
                    <tr className="accounts-detail-row">
                      <td colSpan={COLUMNS.length}>
                        <AccountCard account={acct} />
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}
