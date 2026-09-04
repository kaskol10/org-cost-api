import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { DailyCost } from "../types";

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
  data: DailyCost[];
  title?: string;
}

export default function DailyCostChart({ data, title }: Props) {
  if (!data.length) {
    return <p className="empty">No daily cost data for this period.</p>;
  }

  const hasCost = data.some((d) => d.amount > 0);
  if (!hasCost) {
    return (
      <div>
        {title && <h2>{title}</h2>}
        <p className="empty">
          No EC2-Other spend in this period (Cost Explorer returned $0 for each day).
        </p>
      </div>
    );
  }

  const chartData = data.map((d) => ({
    ...d,
    label: formatDate(d.date),
  }));

  return (
    <div>
      {title && <h2>{title}</h2>}
      <ResponsiveContainer width="100%" height={260}>
        <AreaChart data={chartData} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
          <defs>
            <linearGradient id="costFill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#3dd6c6" stopOpacity={0.35} />
              <stop offset="100%" stopColor="#3dd6c6" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke="#2a3544" strokeDasharray="3 3" vertical={false} />
          <XAxis
            dataKey="label"
            tick={{ fill: "#8b9cb3", fontSize: 11 }}
            axisLine={{ stroke: "#2a3544" }}
            tickLine={false}
          />
          <YAxis
            tick={{ fill: "#8b9cb3", fontSize: 11 }}
            axisLine={false}
            tickLine={false}
            tickFormatter={(v) => `$${v}`}
          />
          <Tooltip
            contentStyle={{
              background: "#1a2330",
              border: "1px solid #2a3544",
              borderRadius: 8,
            }}
            labelStyle={{ color: "#8b9cb3" }}
            formatter={(value: number) => [formatCurrency(value), "EC2-Other"]}
          />
          <Area
            type="monotone"
            dataKey="amount"
            stroke="#3dd6c6"
            strokeWidth={2}
            fill="url(#costFill)"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
