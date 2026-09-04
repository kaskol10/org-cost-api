export function formatCurrency(v: number) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 2,
  }).format(v);
}

export function formatGiB(v: number) {
  return `${v.toLocaleString("en-US", { maximumFractionDigits: 1 })} GiB`;
}

export function formatPercent(v: number) {
  return `${v.toLocaleString("en-US", { maximumFractionDigits: 0 })}%`;
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
