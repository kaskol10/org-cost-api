import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { DailyResourceInventory } from "../types";

function formatDate(iso: string) {
  const d = new Date(iso + "T00:00:00");
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

interface Props {
  data: DailyResourceInventory[];
  title: string;
  countLabel: string;
  color?: string;
}

export default function DailyCountChart({
  data,
  title,
  countLabel,
  color = "#4cc9f0",
}: Props) {
  if (!data.length) {
    return null;
  }

  const chartData = data.map((d) => ({
    ...d,
    label: formatDate(d.date),
  }));

  return (
    <div style={{ marginTop: "1rem" }}>
      <h3 className="category-subhead">{title}</h3>
      <ResponsiveContainer width="100%" height={220}>
        <AreaChart data={chartData} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
          <defs>
            <linearGradient id="countFill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity={0.35} />
              <stop offset="100%" stopColor={color} stopOpacity={0} />
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
            allowDecimals={false}
          />
          <Tooltip
            contentStyle={{
              background: "#1a2330",
              border: "1px solid #2a3544",
              borderRadius: 8,
            }}
            labelStyle={{ color: "#8b9cb3" }}
            formatter={(value: number) => [value.toLocaleString(), countLabel]}
          />
          <Area
            type="monotone"
            dataKey="count"
            stroke={color}
            strokeWidth={2}
            fill="url(#countFill)"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
