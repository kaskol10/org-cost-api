import { describe, expect, it } from "vitest";
import { normalizeMarkdownTables } from "./markdown";

describe("normalizeMarkdownTables", () => {
  it("splits inline header, separator, and rows", () => {
    const input =
      "| Account | GiB | |---|---| | staging | 422 | | production | 918 |";
    const out = normalizeMarkdownTables(input);
    expect(out).toContain("| Account | GiB |");
    expect(out).toContain("|---|---|");
    expect(out).toContain("| staging | 422 |");
    expect(out).toContain("| production | 918 |");
  });

  it("leaves normal prose unchanged", () => {
    const input = "No table here, just text.";
    expect(normalizeMarkdownTables(input)).toBe(input);
  });

  it("handles the waste-by-account inline table shape", () => {
    const input =
      "| Account | Unattached disks | GiB | Est. $/mo | Context | |---|---|---|---|---| | staging | 56 | 422 | $33.76 | debris | | production | 8 | 918 | $73.44 | huge disks |";
    const out = normalizeMarkdownTables(input);
    expect(out.split("\n").length).toBeGreaterThanOrEqual(4);
    expect(out).toContain("| staging | 56 |");
    expect(out).toContain("| production | 8 |");
  });

  it("leaves well-formed multiline tables unchanged", () => {
    const input = "| A | B |\n|---|---|\n| 1 | 2 |";
    expect(normalizeMarkdownTables(input)).toBe(input);
  });
});
