import type { AgentAnalyticsDailyRow } from "@multica/core/types";
import type { DailyTokenData } from "../runtimes/utils";

function formatDateLabel(dateValue: string): string {
  const date = new Date(dateValue + "T00:00:00");
  return `${date.getMonth() + 1}/${date.getDate()}`;
}

export function aggregateAgentAnalyticsDailyTokens(
  rows: readonly AgentAnalyticsDailyRow[],
): DailyTokenData[] {
  const buckets = new Map<string, Omit<DailyTokenData, "label">>();

  for (const row of rows) {
    const bucket = buckets.get(row.date) ?? {
      date: row.date,
      input: 0,
      output: 0,
      cacheRead: 0,
      cacheWrite: 0,
    };
    bucket.input += row.input_tokens;
    bucket.output += row.output_tokens;
    bucket.cacheRead += row.cache_read_tokens;
    bucket.cacheWrite += row.cache_write_tokens;
    buckets.set(row.date, bucket);
  }

  return [...buckets.values()]
    .sort((a, b) => a.date.localeCompare(b.date))
    .map((bucket) => ({
      ...bucket,
      label: formatDateLabel(bucket.date),
    }));
}

export function dailyTokenTotal(rows: readonly DailyTokenData[]): number {
  return rows.reduce(
    (sum, row) => sum + row.input + row.output + row.cacheRead + row.cacheWrite,
    0,
  );
}
