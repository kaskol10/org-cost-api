/**
 * API types, generated from openapi.yaml (see src/generated/openapi.ts).
 *
 * Do not hand-edit shapes here: change the Go response type, update
 * openapi.yaml (the backend contract tests fail until they agree), then run
 * `npm run gen:api`. These aliases only keep the names the UI already uses.
 */
import type { components } from "./generated/openapi";

type Schemas = components["schemas"];

export type DailyCost = Schemas["DailyCost"];
export type UsageTypeCost = Schemas["UsageTypeCost"];
export type APIOperationCost = Schemas["APIOperationCost"];
export type PeeringConnectionDetail = Schemas["PeeringConnectionDetail"];
export type EBSVolumeTypeBreakdown = Schemas["VolumeTypeBreakdown"];
export type EBSVolumeUsageRow = Schemas["VolumeUsageRow"];
export type EBSInstanceVolumeUsage = Schemas["VolumeInstanceUsage"];
export type EBSVolumeUsageSummary = Schemas["VolumeUsageSummary"];
export type EBSVolumeInventory = Schemas["VolumeInventory"];
export type DailyResourceInventory = Schemas["DailyResourceInventory"];
export type EBSVolumeDetail = Schemas["EBSVolumeDetail"];
export type SnapshotTierBreakdown = Schemas["SnapshotTierBreakdown"];
export type EBSSnapshotInventory = Schemas["SnapshotInventoryView"];
export type EBSSnapshotDetail = Schemas["EBSSnapshotDetail"];
export type UsageCategory = Schemas["UsageCategory"];
export type UsageTypeDaily = Schemas["UsageTypeDaily"];
export type SnapshotUsageCost = Schemas["SnapshotUsageCost"];
export type ServiceCost = Schemas["ServiceCost"];
export type CostSummary = Schemas["CostSummary"];
export type SnapshotEvent = Schemas["SnapshotEvent"];
export type SnapshotSummary = Schemas["SnapshotSummary"];
export type AccountDashboard = Schemas["AccountDashboard"];
export type OrgServiceAccountShare = Schemas["OrgServiceAccountShare"];
export type OrgServiceDriver = Schemas["OrgServiceDriver"];
export type ConsolidatedTotals = Schemas["ConsolidatedTotals"];
export type EBSTypeCost = Schemas["EBSTypeCost"];
export type CommitmentCoverage = Schemas["CommitmentCoverage"];
export type TaxBreakdown = Schemas["TaxBreakdown"];
export type DashboardResponse = Schemas["DashboardResponse"];
export type PeriodSummary = Schemas["PeriodSummary"];
export type ChangeSummary = Schemas["ChangeSummary"];
export type ServiceTrend = Schemas["ServiceTrend"];
export type AccountTrend = Schemas["AccountTrend"];
export type Spike = Schemas["Spike"];
export type SpikeMeta = Schemas["SpikeMeta"];
export type TrendsResponse = Schemas["TrendsResponse"];
/**
 * Suggestions as the UI holds them: the API shape, optionally enriched by the
 * org-cost-chat service (POST /v1/suggestions/explain, outside openapi.yaml).
 */
export type Suggestion = Schemas["Suggestion"] & {
  /** LLM-generated explanation (when chat agent enriches suggestions). */
  explanation?: string;
  confidence?: string;
};
export type SuggestionsResponse = Omit<Schemas["SuggestionsResponse"], "suggestions"> & {
  suggestions: Suggestion[];
  /** True when org-cost-chat added LLM explanations. */
  llm_enriched?: boolean;
  /** LLM narrative overview for the suggestions panel. */
  narrative_summary?: string;
  additional_insights?: string[];
};
export type ForecastResult = Schemas["Forecast"];
export type BudgetStatus = Schemas["BudgetStatus"];
export type ReportResponse = Schemas["ReportResponse"];
export type ServerHighlight = Schemas["Highlight"];
export type AskResponse = Schemas["AskResponse"];
export type TagDelta = Schemas["TagDelta"];
export type AccountTagDelta = Schemas["AccountTagDelta"];
export type ServiceTagDeltaResponse = Schemas["ServiceTagDeltaResponse"];
export type ServiceTagTotalsResponse = Schemas["ServiceTagTotalsResponse"];
export type TagBucket = Schemas["TagBucket"];
export type ServiceGroupCost = Schemas["ServiceGroupCost"];
export type ServiceDetail = Schemas["ServiceDetail"];
