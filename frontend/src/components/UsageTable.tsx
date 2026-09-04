function formatCurrency(v: number) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 2,
  }).format(v);
}

interface Row {
  label: string;
  sublabel?: string;
  amount: number;
  percent?: number;
}

interface Props {
  title: string;
  rows: Row[];
  labelHeader?: string;
  showPercent?: boolean;
}

export default function UsageTable({
  title,
  rows,
  labelHeader = "Usage type",
  showPercent = false,
}: Props) {
  if (!rows.length) {
    return (
      <div>
        <h2>{title}</h2>
        <p className="empty">No data.</p>
      </div>
    );
  }

  return (
    <div>
      <h2>{title}</h2>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>{labelHeader}</th>
              {showPercent && <th className="amount">Share</th>}
              <th className="amount">Cost</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.sublabel ? `${r.label}-${r.sublabel}` : r.label}>
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
                {showPercent && (
                  <td className="amount">
                    {r.percent != null ? `${r.percent.toFixed(1)}%` : "—"}
                  </td>
                )}
                <td className="amount">{formatCurrency(r.amount)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
