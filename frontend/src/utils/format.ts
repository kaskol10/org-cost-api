export function formatCurrency(v: number) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 2,
  }).format(v);
}

/** Signed currency for deltas: +$1.20 / −$1.20 (ASCII hyphen-minus for −). */
export function formatSignedCurrency(v: number) {
  const abs = formatCurrency(Math.abs(v));
  if (v > 0) return `+${abs}`;
  if (v < 0) return `-${abs}`;
  return abs;
}

export function formatPercent(v: number) {
  return `${v.toLocaleString("en-US", { maximumFractionDigits: 0 })}%`;
}

export function formatSignedPercent(v: number) {
  const abs = formatPercent(Math.abs(v));
  if (v > 0) return `+${abs}`;
  if (v < 0) return `-${abs}`;
  return abs;
}

export type SpendDirection = "up" | "down" | "flat";

/** Classify spend movement; tiny moves count as flat. */
export function spendDirection(changeUsd: number, changePercent: number): SpendDirection {
  if (Math.abs(changeUsd) < 1 && Math.abs(changePercent) < 0.5) return "flat";
  if (changeUsd > 0 || (changeUsd === 0 && changePercent > 0)) return "up";
  if (changeUsd < 0 || changePercent < 0) return "down";
  return "flat";
}

/** Hero pill: "+$1.2k · +8% higher spend vs …" */
export function formatSpendChangePill(
  changeUsd: number,
  changePercent: number,
  compareLabel: string
): string {
  const dir = spendDirection(changeUsd, changePercent);
  if (dir === "flat") return `About flat ${compareLabel}`;
  const word = dir === "up" ? "higher spend" : "lower spend";
  return `${formatSignedCurrency(changeUsd)} · ${formatSignedPercent(changePercent)} ${word} ${compareLabel}`;
}

/** Prior line detail after the prior total. */
export function formatSpendChangeDetail(changeUsd: number, changePercent: number): string {
  const dir = spendDirection(changeUsd, changePercent);
  if (dir === "flat") return "about flat vs prior";
  const word = dir === "up" ? "higher spend" : "lower spend";
  return `${formatSignedCurrency(changeUsd)} ${word} (${formatSignedPercent(changePercent)})`;
}

/** Service row delta: drove +$X / saved $X (realized period move, not opportunity). */
export function formatServiceSpendDelta(changeUsd: number, changePercent: number): string {
  const dir = spendDirection(changeUsd, changePercent);
  if (dir === "flat") return `flat (${formatSignedPercent(changePercent)})`;
  if (dir === "up") {
    return `drove ${formatSignedCurrency(changeUsd)} (${formatSignedPercent(changePercent)})`;
  }
  return `saved ${formatCurrency(Math.abs(changeUsd))} (${formatSignedPercent(changePercent)})`;
}

export function formatGiB(v: number) {
  return `${v.toLocaleString("en-US", { maximumFractionDigits: 1 })} GiB`;
}

export function formatCountSize(count: number, sizeGiB: number, noun: string) {
  return `${count.toLocaleString()} ${noun} · ${formatGiB(sizeGiB)}`;
}

export function formatServiceName(service: string) {
  const s = (service ?? "").trim();
  if (!s) return "Unknown";

  // Common Cost Explorer SERVICE dimension values.
  const exact: Record<string, string> = {
    "Amazon Simple Storage Service": "S3",
    "AWS Data Transfer": "Data transfer",
    "Amazon Redshift": "Redshift",
    "Amazon Relational Database Service": "RDS",
    "Amazon Elastic Compute Cloud - Compute": "EC2",
    "Amazon Elastic Kubernetes Service": "EKS",
    "Elastic Load Balancing": "ELB",
    "Amazon CloudWatch": "CloudWatch",
    "AWS Lambda": "Lambda",
    "Amazon Kinesis": "Kinesis",
  };
  if (exact[s]) return exact[s];

  // Catch common variants.
  if (s.includes("Amazon Simple Storage Service")) return "S3";
  if (s.includes("Amazon Kinesis")) return "Kinesis";
  if (s.includes("Amazon Redshift")) return "Redshift";
  if (s.includes("Relational Database Service")) return "RDS";
  if (s.includes("CloudWatch")) return "CloudWatch";
  if (s.includes("Elastic Load Balancing")) return "ELB";
  if (s.includes("Kubernetes Service")) return "EKS";
  if (s.includes("Data Transfer")) return "Data transfer";

  return s;
}
