import { describe, expect, it } from "vitest";
import { readUrlState, writeUrlState } from "./urlState";

describe("url state", () => {
  it("reads defaults from an empty query", () => {
    expect(readUrlState("")).toEqual({ period: "30d", service: null, account: null });
  });

  it("round-trips a drill-down", () => {
    const state = { period: "mtd" as const, service: "Amazon Relational Database Service", account: "111111111111" };
    const search = writeUrlState(state);
    expect(search).toBe("?period=mtd&service=Amazon+Relational+Database+Service&account=111111111111");
    expect(readUrlState(search)).toEqual(state);
  });

  it("omits defaults and keeps unrelated params", () => {
    expect(writeUrlState({ period: "30d", service: null, account: null }, "?period=mtd&utm=x")).toBe("?utm=x");
  });

  it("ignores unknown periods", () => {
    expect(readUrlState("?period=7d").period).toBe("30d");
  });
});
