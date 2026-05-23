import { describe, expect, it } from "vitest";
import type { AgentAnalyticsDailyRow } from "@multica/core/types";
import {
  aggregateAgentAnalyticsDailyTokens,
  dailyTokenTotal,
} from "./agent-analytics-utils";

function dailyRow(
  date: string,
  overrides: Partial<AgentAnalyticsDailyRow>,
): AgentAnalyticsDailyRow {
  return {
    date,
    provider: "openai",
    model: "gpt-5.4",
    input_tokens: 0,
    output_tokens: 0,
    cache_read_tokens: 0,
    cache_write_tokens: 0,
    total_tokens: 0,
    task_count: 1,
    completed_count: 1,
    failed_count: 0,
    cancelled_count: 0,
    ...overrides,
  };
}

describe("agent analytics chart data", () => {
  it("aggregates per-model daily rows into the Usage token chart shape", () => {
    const rows = [
      dailyRow("2026-05-22", {
        model: "gpt-5.4",
        input_tokens: 100,
        output_tokens: 20,
        cache_read_tokens: 300,
      }),
      dailyRow("2026-05-21", {
        input_tokens: 10,
        cache_write_tokens: 5,
      }),
      dailyRow("2026-05-22", {
        model: "gpt-5.4-mini",
        input_tokens: 7,
        output_tokens: 3,
        cache_write_tokens: 2,
      }),
    ];

    const chartRows = aggregateAgentAnalyticsDailyTokens(rows);

    expect(chartRows).toEqual([
      {
        date: "2026-05-21",
        label: "5/21",
        input: 10,
        output: 0,
        cacheRead: 0,
        cacheWrite: 5,
      },
      {
        date: "2026-05-22",
        label: "5/22",
        input: 107,
        output: 23,
        cacheRead: 300,
        cacheWrite: 2,
      },
    ]);
    expect(dailyTokenTotal(chartRows)).toBe(447);
  });
});
