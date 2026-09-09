import { describe, expect, it } from "vitest";
import {
  formatServiceSpendDelta,
  formatSpendChangeDetail,
  formatSpendChangePill,
  spendDirection,
} from "./format";

describe("spendDirection", () => {
  it("classifies up / down / flat", () => {
    expect(spendDirection(1200, 8)).toBe("up");
    expect(spendDirection(-900, -6)).toBe("down");
    expect(spendDirection(0.2, 0.1)).toBe("flat");
  });
});

describe("formatSpendChangePill", () => {
  it("uses higher / lower spend wording", () => {
    expect(formatSpendChangePill(1200, 8, "vs same days last month")).toBe(
      "+$1,200.00 · +8% higher spend vs same days last month"
    );
    expect(formatSpendChangePill(-900, -6, "vs prior period")).toBe(
      "-$900.00 · -6% lower spend vs prior period"
    );
    expect(formatSpendChangePill(0.2, 0.1, "vs prior period")).toBe(
      "About flat vs prior period"
    );
  });
});

describe("formatSpendChangeDetail", () => {
  it("describes the dollar move without saying savings opportunity", () => {
    expect(formatSpendChangeDetail(500, 4)).toBe("+$500.00 higher spend (+4%)");
    expect(formatSpendChangeDetail(-500, -4)).toBe("-$500.00 lower spend (-4%)");
  });
});

describe("formatServiceSpendDelta", () => {
  it("labels rows as drove / saved", () => {
    expect(formatServiceSpendDelta(200, 10)).toBe("drove +$200.00 (+10%)");
    expect(formatServiceSpendDelta(-150, -5)).toBe("saved $150.00 (-5%)");
  });
});
