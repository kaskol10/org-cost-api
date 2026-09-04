import type { OrgServiceDriver } from "../types";
import { formatCurrency, formatServiceName } from "../utils/format";

interface Props {
  drivers: OrgServiceDriver[];
  orgTotal: number;
}

export default function OrgTopServicesPanel({ drivers, orgTotal }: Props) {
  if (!drivers.length) {
    return (
      <section className="panel">
        <h2>Top cost drivers (organization)</h2>
        <p className="empty">No service cost data for this period.</p>
      </section>
    );
  }

  return (
    <section className="panel org-drivers-panel">
      <h2>Top cost drivers (organization)</h2>
      <p className="category-desc" style={{ marginTop: "0.35rem", marginBottom: "1rem" }}>
        Aggregated across all configured accounts · {formatCurrency(orgTotal)} total usage
      </p>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Service</th>
              <th className="amount">Share</th>
              <th className="amount">Cost</th>
              <th>Top accounts</th>
            </tr>
          </thead>
          <tbody>
            {drivers.map((d) => {
              const label = formatServiceName(d.service);
              const raw = label !== d.service ? d.service : null;
              return (
                <tr key={d.service}>
                  <td>
                    <div className="mono" style={{ fontWeight: 500 }}>
                      {label}
                    </div>
                    {raw && (
                      <div className="mono" style={{ fontSize: "0.8rem", color: "var(--muted)" }}>
                        {raw}
                      </div>
                    )}
                  </td>
                  <td className="amount">
                    {d.percent != null ? `${d.percent.toFixed(1)}%` : "—"}
                  </td>
                  <td className="amount">{formatCurrency(d.amount)}</td>
                  <td>
                    {d.top_accounts && d.top_accounts.length > 0 ? (
                      <ul className="org-driver-accounts">
                        {d.top_accounts.map((a) => (
                          <li key={a.account_id}>
                            <span className="mono">{a.account_name}</span>
                            <span className="org-driver-acct-meta">
                              {formatCurrency(a.amount)}
                              {a.percent != null && (
                                <span style={{ color: "var(--muted)" }}>
                                  {" "}
                                  · {a.percent.toFixed(0)}%
                                </span>
                              )}
                            </span>
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <span style={{ color: "var(--muted)" }}>—</span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}
