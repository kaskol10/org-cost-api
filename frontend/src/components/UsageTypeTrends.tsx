import {
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { UsageTypeDaily } from "../types";

const CATEGORY_LABELS: Record<string, string> = {
  "ebs-volumes": "EBS volumes",
  "ebs-snapshots": "EBS snapshots",
  "nat-gateway": "NAT Gateway",
  "vpc-peering": "VPC peering",
  "data-transfer": "Data transfer",
  "elastic-ip": "Elastic IP",
  "cpu-credits": "CPU credits",
  other: "Other",
};

const COLORS = [
  "#3dd6c6",
  "#4cc9f0",
  "#f4a261",
  "#e9c46a",
  "#b5179e",
  "#90be6d",
  "#f94144",
  "#577590",
];

function formatCurrency(v: number) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 2,
  }).format(v);
}

function formatDate(iso: string) {
  const d = new Date(iso + "T00:00:00");
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

interface Props {
  series: UsageTypeDaily[];
}

export default function UsageTypeTrends({ series }: Props) {
  if (!series.length) {
    return null;
  }

  return (
    <div>
      <h2>Daily trend — top usage types</h2>
      <div className="usage-trends-grid">
        {series.map((s, i) => {
          const data = s.daily.map((d) => ({
            ...d,
            label: formatDate(d.date),
          }));
          const color = COLORS[i % COLORS.length];
          return (
            <div key={s.usage_type} className="usage-trend-card">
              <div className="usage-trend-title">
                <span className="mono">{s.usage_type}</span>
                <span className="usage-trend-badge">
                  {CATEGORY_LABELS[s.category] ?? s.category}
                </span>
              </div>
              <div className="usage-trend-total">{formatCurrency(s.total)}</div>
              <ResponsiveContainer width="100%" height={120}>
                <LineChart data={data} margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
                  <XAxis
                    dataKey="label"
                    tick={{ fill: "#8b9cb3", fontSize: 9 }}
                    axisLine={false}
                    tickLine={false}
                    interval="preserveStartEnd"
                  />
                  <YAxis
                    tick={{ fill: "#8b9cb3", fontSize: 9 }}
                    axisLine={false}
                    tickLine={false}
                    width={36}
                    tickFormatter={(v) => `$${v}`}
                  />
                  <Tooltip
                    contentStyle={{
                      background: "#1a2330",
                      border: "1px solid #2a3544",
                      borderRadius: 8,
                      fontSize: 12,
                    }}
                    formatter={(value: number) => [formatCurrency(value), "Cost"]}
                  />
                  <Line
                    type="monotone"
                    dataKey="amount"
                    stroke={color}
                    strokeWidth={2}
                    dot={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            </div>
          );
        })}
      </div>
    </div>
  );
}
