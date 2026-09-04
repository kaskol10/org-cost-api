import { describe, expect, it } from "vitest";
import type { Suggestion } from "../types";
import { groupSuggestionsByCategory, summarizeSuggestions } from "./suggestions";

const SAMPLE: Suggestion[] = [
  {
    id: "unattached-ebs",
    priority: 1,
    category: "waste",
    title: "EBS waste",
    detail: "detail",
    estimated_monthly_usd: 8,
    actions: [],
  },
  {
    id: "spike-redshift",
    priority: 2,
    category: "trend",
    title: "Redshift spike",
    detail: "detail",
    actions: [],
  },
  {
    id: "concentration",
    priority: 3,
    category: "concentration",
    title: "S3 concentration",
    detail: "detail",
    estimated_monthly_usd: 0,
    actions: [],
  },
];

describe("summarizeSuggestions", () => {
  it("totals quantified savings and counts by category", () => {
    const s = summarizeSuggestions(SAMPLE);
    expect(s.totalCount).toBe(3);
    expect(s.quantifiedCount).toBe(1);
    expect(s.totalQuantifiedSavingsUsd).toBe(8);
    expect(s.byCategory.map((c) => c.category)).toEqual([
      "waste",
      "trend",
      "concentration",
    ]);
    expect(s.byCategory[0].quantifiedSavingsUsd).toBe(8);
  });
});

describe("groupSuggestionsByCategory", () => {
  it("groups in category order", () => {
    const groups = groupSuggestionsByCategory(SAMPLE);
    expect(groups).toHaveLength(3);
    expect(groups[0].category.label).toBe("Waste");
    expect(groups[0].items).toHaveLength(1);
  });
});
