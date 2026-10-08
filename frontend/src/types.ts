export interface DailyCost {
  date: string;
  amount: number;
  unit: string;
}

export interface UsageTypeCost {
  usage_type: string;
  region?: string;
  short_name?: string;
  amount: number;
  unit: string;
  percent?: number;
}

export interface APIOperationCost {
  operation: string;
  amount: number;
  unit: string;
  percent?: number;
}

export interface PeeringConnectionDetail {
  peering_id: string;
  name: string;
  status: string;
  region: string;
  local_vpc: string;
  local_cidr?: string;
  peer_vpc: string;
  peer_cidr?: string;
  peer_region?: string;
  peer_account?: string;
  local_role: string;
  out_total: number;
  in_total: number;
  out_vpc?: string;
  in_vpc?: string;
  attribution_note?: string;
}

export interface EBSVolumeTypeBreakdown {
  volume_type: string;
  count: number;
  size_gib: number;
}

export interface EBSVolumeUsageRow {
  volume_id: string;
  instance_id?: string;
  volume_type: string;
  state: string;
  provisioned_gib: number;
  filesystem_used_gib?: number;
  utilization_percent?: number;
  note?: string;
}

export interface EBSInstanceVolumeUsage {
  instance_id: string;
  volume_count: number;
  provisioned_gib: number;
  filesystem_used_gib?: number;
  filesystem_total_gib?: number;
  utilization_percent?: number;
}

export interface EBSVolumeUsageSummary {
  provisioned_gib: number;
  attached_provisioned_gib: number;
  unattached_provisioned_gib: number;
  filesystem_used_gib?: number;
  utilization_percent?: number;
  measured_provisioned_gib: number;
  coverage_percent: number;
  usage_note?: string;
  by_instance?: EBSInstanceVolumeUsage[];
  top_underutilized?: EBSVolumeUsageRow[];
}

export interface EBSVolumeInventory {
  count: number;
  available_count?: number;
  available_gib?: number;
  total_gib: number;
  by_type: EBSVolumeTypeBreakdown[];
  usage?: EBSVolumeUsageSummary;
}

export interface DailyResourceInventory {
  date: string;
  count: number;
  usage_amount?: number;
}

export interface EBSVolumeDetail {
  inventory?: EBSVolumeInventory;
  daily_cost: DailyCost[];
  cur_daily?: DailyResourceInventory[];
  inventory_note?: string;
}

export interface SnapshotTierBreakdown {
  tier: string;
  count: number;
  size_gib: number;
}

export interface EBSSnapshotInventory {
  count: number;
  total_size_gib: number;
  by_tier?: SnapshotTierBreakdown[];
}

export interface EBSSnapshotDetail {
  inventory?: EBSSnapshotInventory;
  daily_cost?: DailyCost[];
  cur_daily?: DailyResourceInventory[];
  inventory_note?: string;
}

export interface UsageCategory {
  id: string;
  label: string;
  description: string;
  amount: number;
  percent: number;
  usage_types: UsageTypeCost[];
  api_operations?: APIOperationCost[];
  peering_details?: PeeringConnectionDetail[];
  peering_drivers_note?: string;
  ebs_volume_detail?: EBSVolumeDetail;
  ebs_snapshot_detail?: EBSSnapshotDetail;
  inventory_count?: number;
  inventory_size_gib?: number;
}

export interface UsageTypeDaily {
  usage_type: string;
  category: string;
  total: number;
  daily: DailyCost[];
}

export interface SnapshotUsageCost {
  usage_type: string;
  amount: number;
  unit: string;
}

export interface ServiceCost {
  service: string;
  amount: number;
  unit: string;
  percent?: number;
}

export interface CostSummary {
  account_id: string;
  account_name: string;
  start: string;
  end: string;
  total: number;
  unit: string;
  daily: DailyCost[];
  by_usage_type: UsageTypeCost[];
  usage_categories: UsageCategory[];
  top_usage_daily: UsageTypeDaily[];
  by_api_operation: APIOperationCost[];
  snapshot_usage: SnapshotUsageCost[];

  all_total: number;
  all_daily: DailyCost[];
  by_service: ServiceCost[];
  other_services_total: number;
}

export interface SnapshotEvent {
  snapshot_id: string;
  volume_id: string;
  size_gib: number;
  start_time: string;
  description?: string;
}

export interface SnapshotSummary {
  account_id: string;
  account_name: string;
  region: string;
  count: number;
  total_size_gib: number;
  total_size_bytes: number;
  scanned_at: string;
  by_tier?: SnapshotTierBreakdown[];
  recent_creates?: SnapshotEvent[];
}

export interface AccountDashboard {
  account_id: string;
  account_name: string;
  error?: string;
  costs?: CostSummary;
  volumes?: EBSVolumeInventory;
  snapshots?: SnapshotSummary;
}

export interface OrgServiceAccountShare {
  account_id: string;
  account_name: string;
  amount: number;
  percent?: number;
}

export interface OrgServiceDriver {
  service: string;
  amount: number;
  unit: string;
  percent?: number;
  top_accounts?: OrgServiceAccountShare[];
}

export interface ConsolidatedTotals {
  org_total: number;
  ec2_other_cost: number;
  unit: string;
  volume_count: number;
  volume_available_count: number;
  volume_available_gib?: number;
  volume_size_gib: number;
  volume_filesystem_used_gib?: number;
  volume_utilization_percent?: number;
  volume_usage_coverage_percent?: number;
  snapshot_count: number;
  snapshot_size_gib: number;
  account_count: number;
}

export interface CommitmentCoverage {
  sp_coverage_pct?: number;
  sp_utilization_pct?: number;
  ri_coverage_pct?: number;
  uncommitted_usd?: number;
  has_commitments?: boolean;
}

// Tax isolates Tax records (RECORD_TYPE=Tax) from usage. AWS posts the
// previous month's tax as a lump on the 1st of the month, which otherwise
// fakes a day-1 spike and skews period-over-period comparisons.
export interface TaxBreakdown {
  total_usd: number;
  /** Org usage+tax spend (excludes only credits/refunds). */
  incl_tax_total_usd?: number;
  by_service?: OrgServiceDriver[];
  daily?: DailyCost[];
  posted_days?: number;
  has_data?: boolean;
}

export interface DashboardResponse {
  generated_at: string;
  start: string;
  end: string;
  accounts: AccountDashboard[];
  totals: ConsolidatedTotals;
  top_services?: OrgServiceDriver[];
  cur_enabled?: boolean;
  cur_note?: string;
  incomplete_days?: number;
  tax_excluded?: boolean;
  tax?: TaxBreakdown;
  commitments?: CommitmentCoverage;
}

export interface PeriodSummary {
  start: string;
  end: string;
  days?: number;
}

export interface ChangeSummary {
  current_usd: number;
  prior_usd: number;
  change_usd: number;
  change_percent: number;
}

export interface ServiceTrend {
  service: string;
  display_name: string;
  current_usd: number;
  prior_usd: number;
  change_usd: number;
  change_percent: number;
  direction: string;
  current_share_pct?: number;
}

export interface AccountTrend {
  account_id: string;
  account_name: string;
  current_usd: number;
  prior_usd: number;
  change_usd: number;
  change_percent: number;
  direction: string;
  current_share_pct?: number;
}

export interface Spike {
  date: string;
  amount_usd: number;
  baseline_usd: number;
  deviation_pct: number;
  direction: string;
  account_ids?: string[];
  incomplete?: boolean;
}

export interface TrendsResponse {
  generated_at: string;
  period?: "30d" | "mtd" | string;
  current_period: PeriodSummary;
  prior_period: PeriodSummary;
  prior_source: string;
  ce_calls_used: number;
  org_total: ChangeSummary;
  service_trends?: ServiceTrend[];
  top_increases: ServiceTrend[];
  top_decreases: ServiceTrend[];
  account_trends?: AccountTrend[];
  top_account_increases?: AccountTrend[];
  top_account_decreases?: AccountTrend[];
  spikes?: Spike[];
  history_note?: string;
  snapshot_count?: number;
  refresh_allowed?: boolean;
}

export interface Suggestion {
  id: string;
  priority: number;
  category: string;
  title: string;
  detail: string;
  estimated_monthly_usd?: number;
  actions: string[];
  account?: string;
  service?: string;
  /** LLM-generated explanation (when chat agent enriches suggestions). */
  explanation?: string;
  confidence?: string;
}

export interface SuggestionsResponse {
  generated_at: string;
  period: string;
  suggestions: Suggestion[];
  summary: string;
  ce_calls_used: number;
  data_sources: string[];
  /** True when org-cost-chat added LLM explanations. */
  llm_enriched?: boolean;
  /** LLM narrative overview for the suggestions panel. */
  narrative_summary?: string;
  additional_insights?: string[];
}

export interface ForecastResult {
  projected_usd: number;
  mtd_usd: number;
  days_elapsed: number;
  days_in_month: number;
  daily_average: number;
  method?: string;
}

export interface BudgetStatus {
  name: string;
  monthly_usd: number;
  account?: string;
  projected_usd: number;
  percent_of_budget: number;
  status: "ok" | "warning" | "over";
  over_by_usd?: number;
}

export interface ReportResponse {
  dashboard: DashboardResponse;
  trends: TrendsResponse;
  suggestions: SuggestionsResponse;
  ce_calls_used: number;
  refresh_allowed: boolean;
  forecast?: ForecastResult;
  budgets?: BudgetStatus[];
}

export interface AskResponse {
  answer: string;
  intent: string;
  ce_calls_used: number;
  sources: string[];
}

export interface TagDelta {
  key: string;
  current_usd: number;
  prior_usd: number;
  change_usd: number;
  change_percent: number;
  current_share_pct?: number;
}

export interface AccountTagDelta {
  account_id: string;
  account_name: string;
  total_current_usd: number;
  total_prior_usd: number;
  delta_usd: number;
  delta_percent: number;
  top_increases: TagDelta[];
  top_decreases: TagDelta[];
}

export interface ServiceTagDeltaResponse {
  generated_at: string;
  service: string;
  display_name: string;
  tag_key: string;
  current_period: PeriodSummary;
  prior_period: PeriodSummary;
  total_current_usd: number;
  total_prior_usd: number;
  delta_usd: number;
  delta_percent: number;
  top_increases: TagDelta[];
  top_decreases: TagDelta[];
  accounts: AccountTagDelta[];
  top_account_note?: string;
  ce_calls_used: number;
}

export interface ServiceTagTotalsResponse {
  generated_at: string;
  service: string;
  display_name: string;
  tag_key: string;
  current_period: PeriodSummary;
  total_current_usd: number;
  buckets: TagBucket[];
  ce_calls_used: number;
}

export interface TagBucket {
  key: string;
  current_usd: number;
  current_share_pct?: number;
}

export interface ServiceGroupCost {
  key: string;
  amount: number;
  unit: string;
  percent?: number;
}

export interface ServiceDetail {
  account_id: string;
  service: string;
  start: string;
  end: string;
  total: number;
  unit: string;
  daily: DailyCost[];
  by_region: ServiceGroupCost[];
  by_usage_type: ServiceGroupCost[];
  by_operation: ServiceGroupCost[];

  by_name_tag?: ServiceGroupCost[];
}
