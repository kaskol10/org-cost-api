/**
 * Normalize LLM output where markdown tables are collapsed onto one line.
 *
 * Example:
 *   | A | B | |---|---| | x | y |
 * becomes:
 *   | A | B |
 *   |---|---|
 *   | x | y |
 */
export function normalizeMarkdownTables(markdown: string): string {
  return markdown
    .split("\n")
    .map((line) => normalizeInlineTableLine(line))
    .join("\n");
}

function normalizeInlineTableLine(line: string): string {
  if (!line.includes("|") || !/-{3,}/.test(line)) {
    return line;
  }
  const pipeCount = (line.match(/\|/g) || []).length;
  if (pipeCount < 4) {
    return line;
  }

  let out = line;
  // Header row -> separator row (e.g. "| Context | |---|---|")
  out = out.replace(/\|\s+\|([-:\s|]*-{3,}[-:\s|]*\|)/g, "|\n|$1");
  // Data rows (e.g. "|---| | staging |" or "| debris | | production |")
  out = out.replace(/\|\s+\|\s+([A-Za-z0-9_*])/g, "|\n| $1");
  return out;
}
