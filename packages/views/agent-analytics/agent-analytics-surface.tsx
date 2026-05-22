"use client";

import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  BarChart3,
  ChevronDown,
  ChevronRight,
  CircleDollarSign,
  Database,
  Gauge,
  Settings2,
} from "lucide-react";
import { agentAnalyticsRunsOptions } from "@multica/core/agent-analytics";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type {
  AgentAnalyticsModelUsage,
  AgentAnalyticsResponse,
  AgentAnalyticsRunRow,
  AgentAnalyticsSort,
  AgentAnalyticsSourceFilter,
  AgentAnalyticsSummary,
  GetAgentAnalyticsParams,
} from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@multica/ui/components/ui/empty";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@multica/ui/components/ui/table";
import { cn } from "@multica/ui/lib/utils";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { CustomPricingDialog } from "../runtimes/components/custom-pricing-dialog";
import {
  collectUnmappedModels,
  estimateCost,
  estimateCostBreakdown,
  formatTokens,
  isModelPriced,
} from "../runtimes/utils";

type AgentAnalyticsScope =
  | { kind: "project"; projectId: string }
  | { kind: "chat"; sessionId: string };

type PeriodValue = 7 | 30 | 90 | "all";

const PAGE_SIZE = 50;

export function AgentAnalyticsSurface({
  scope,
  active,
}: {
  scope: AgentAnalyticsScope;
  active: boolean;
}) {
  const { t } = useT("usage");
  const wsId = useWorkspaceId();
  const [period, setPeriod] = useState<PeriodValue>(
    scope.kind === "project" ? 30 : "all",
  );
  const [source, setSource] = useState<AgentAnalyticsSourceFilter>("all");
  const [sort, setSort] = useState<AgentAnalyticsSort>("newest");
  const [offset, setOffset] = useState(0);
  const [pricingOpen, setPricingOpen] = useState(false);

  const params = useMemo<GetAgentAnalyticsParams>(
    () => ({
      scope: scope.kind,
      scope_id: scope.kind === "project" ? scope.projectId : scope.sessionId,
      days: period,
      source: scope.kind === "project" ? source : "all",
      limit: PAGE_SIZE,
      offset,
      sort,
    }),
    [offset, period, scope, sort, source],
  );
  const query = useQuery(agentAnalyticsRunsOptions(wsId, params, active));
  const data = query.data;
  const summary = data?.summary;
  const unmappedModels = useMemo(
    () => collectUnmappedModels(summary?.model_usage ?? []),
    [summary?.model_usage],
  );

  const updatePeriod = (next: PeriodValue) => {
    setPeriod(next);
    setOffset(0);
  };
  const updateSource = (next: AgentAnalyticsSourceFilter) => {
    setSource(next);
    setOffset(0);
  };
  const updateSort = (next: AgentAnalyticsSort) => {
    setSort((current) => (current === next ? "newest" : next));
    setOffset(0);
  };

  if (!active) return null;

  if (query.isLoading) {
    return <AgentAnalyticsSkeleton />;
  }

  if (query.isError) {
    return (
      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
        <Empty className="min-h-80 border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <BarChart3 />
            </EmptyMedia>
            <EmptyTitle>{t(($) => $.analytics.error_title)}</EmptyTitle>
            <EmptyDescription>{t(($) => $.analytics.error_description)}</EmptyDescription>
          </EmptyHeader>
        </Empty>
      </div>
    );
  }

  if (!data || data.summary.task_count === 0) {
    return (
      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
        <AnalyticsControls
          scopeKind={scope.kind}
          period={period}
          source={source}
          onPeriodChange={updatePeriod}
          onSourceChange={updateSource}
        />
        <Empty className="mt-4 min-h-80 border border-dashed">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <BarChart3 />
            </EmptyMedia>
            <EmptyTitle>{t(($) => $.analytics.empty_title)}</EmptyTitle>
            <EmptyDescription>{t(($) => $.analytics.empty_description)}</EmptyDescription>
          </EmptyHeader>
        </Empty>
      </div>
    );
  }

  return (
    <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-4">
        <AnalyticsControls
          scopeKind={scope.kind}
          period={period}
          source={source}
          onPeriodChange={updatePeriod}
          onSourceChange={updateSource}
        />
        <AnalyticsGaugeRow
          data={data}
          unmappedModels={unmappedModels}
          onConfigurePricing={() => setPricingOpen(true)}
        />
        <AnalyticsSecondaryPanels data={data} />
        <AnalyticsRunTable
          data={data}
          sort={sort}
          onSortChange={updateSort}
          onPageChange={setOffset}
        />
      </div>
      <CustomPricingDialog
        open={pricingOpen}
        onOpenChange={setPricingOpen}
        unmappedModels={unmappedModels}
      />
    </div>
  );
}

function AnalyticsControls({
  scopeKind,
  period,
  source,
  onPeriodChange,
  onSourceChange,
}: {
  scopeKind: AgentAnalyticsScope["kind"];
  period: PeriodValue;
  source: AgentAnalyticsSourceFilter;
  onPeriodChange: (value: PeriodValue) => void;
  onSourceChange: (value: AgentAnalyticsSourceFilter) => void;
}) {
  const { t } = useT("usage");
  const periods: PeriodValue[] =
    scopeKind === "project" ? [7, 30, 90] : ["all", 7, 30, 90];
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <SegmentedControl
          label={t(($) => $.analytics.controls.period)}
          items={periods.map((value) => ({
            value,
            label: value === "all" ? t(($) => $.analytics.controls.all) : `${value}d`,
          }))}
          value={period}
          onChange={onPeriodChange}
        />
        {scopeKind === "project" && (
          <SegmentedControl
            label={t(($) => $.analytics.controls.source)}
            items={[
              { value: "all", label: t(($) => $.analytics.controls.source_all) },
              { value: "issues", label: t(($) => $.analytics.controls.source_issues) },
              { value: "chats", label: t(($) => $.analytics.controls.source_chats) },
            ]}
            value={source}
            onChange={onSourceChange}
          />
        )}
      </div>
    </div>
  );
}

function SegmentedControl<T extends string | number>({
  label,
  items,
  value,
  onChange,
}: {
  label: string;
  items: Array<{ value: T; label: string }>;
  value: T;
  onChange: (value: T) => void;
}) {
  return (
    <div className="flex items-center gap-2">
      <span className="text-xs text-muted-foreground">{label}</span>
      <div className="inline-flex rounded-lg border bg-background p-0.5">
        {items.map((item) => (
          <Button
            key={String(item.value)}
            type="button"
            variant={item.value === value ? "secondary" : "ghost"}
            size="xs"
            className="h-6 rounded-md px-2"
            onClick={() => onChange(item.value)}
          >
            {item.label}
          </Button>
        ))}
      </div>
    </div>
  );
}

function AnalyticsGaugeRow({
  data,
  unmappedModels,
  onConfigurePricing,
}: {
  data: AgentAnalyticsResponse;
  unmappedModels: string[];
  onConfigurePricing: () => void;
}) {
  const { t } = useT("usage");
  const current = metricValues(data.summary);
  const previous = data.previous_summary ? metricValues(data.previous_summary) : null;
  const unpricedTokens = unpricedTokenTotal(data.summary.model_usage);

  return (
    <div className="grid gap-3 md:grid-cols-3">
      <MetricTile
        icon={<Database />}
        label={t(($) => $.analytics.gauges.tokens)}
        value={formatTokens(data.summary.total_tokens)}
        hint={t(($) => $.analytics.gauges.tokens_hint, {
          input: formatTokens(data.summary.input_tokens),
          output: formatTokens(data.summary.output_tokens),
        })}
        delta={previous ? deltaText(current.tokens, previous.tokens) : null}
      />
      <MetricTile
        icon={<CircleDollarSign />}
        label={t(($) => $.analytics.gauges.cost)}
        value={formatCost(current.cost)}
        hint={
          unpricedTokens > 0
            ? t(($) => $.analytics.gauges.unpriced_tokens, {
                tokens: formatTokens(unpricedTokens),
              })
            : t(($) => $.analytics.gauges.cost_hint)
        }
        action={
          unmappedModels.length > 0 ? (
            <Button size="xs" variant="outline" onClick={onConfigurePricing}>
              <Settings2 className="size-3" />
              {t(($) => $.analytics.gauges.configure_pricing)}
            </Button>
          ) : null
        }
        delta={previous ? deltaText(current.cost, previous.cost) : null}
      />
      <MetricTile
        icon={<Gauge />}
        label={t(($) => $.analytics.gauges.speed)}
        value={formatDuration(data.summary.median_completion_ms)}
        hint={t(($) => $.analytics.gauges.speed_hint, {
          p95: formatDuration(data.summary.p95_completion_ms),
        })}
        delta={previous ? deltaText(current.speedMs, previous.speedMs) : null}
      />
    </div>
  );
}

function MetricTile({
  icon,
  label,
  value,
  hint,
  action,
  delta,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  hint: string;
  action?: ReactNode;
  delta: string | null;
}) {
  return (
    <div className="min-w-0 rounded-lg border bg-background p-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2 text-xs font-medium text-muted-foreground">
          <span className="flex size-6 items-center justify-center rounded-md bg-muted text-foreground [&_svg]:size-3.5">
            {icon}
          </span>
          <span className="truncate">{label}</span>
        </div>
        {delta && <span className="shrink-0 text-xs text-muted-foreground">{delta}</span>}
      </div>
      <div className="mt-3 min-w-0 text-2xl font-semibold tracking-normal">{value}</div>
      <div className="mt-1 flex min-h-6 items-center justify-between gap-2 text-xs text-muted-foreground">
        <span className="min-w-0 truncate">{hint}</span>
        {action}
      </div>
    </div>
  );
}

function AnalyticsSecondaryPanels({ data }: { data: AgentAnalyticsResponse }) {
  const { t } = useT("usage");
  const breakdown = costBreakdown(data.summary.model_usage);
  const topSources = (data.sources ?? []).slice(0, 5);
  const daily = dailyTokenBuckets(data).slice(-14);
  const maxDailyTokens = Math.max(...daily.map((row) => row.total_tokens), 1);

  return (
    <div className="grid gap-3 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
      <section className="rounded-lg border bg-background p-3">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-sm font-medium">{t(($) => $.analytics.trend.title)}</h2>
          <span className="text-xs text-muted-foreground">
            {t(($) => $.analytics.trend.caption)}
          </span>
        </div>
        <div className="mt-4 flex h-32 items-end gap-1.5">
          {daily.length === 0 ? (
            <div className="flex h-full flex-1 items-center justify-center text-xs text-muted-foreground">
              {t(($) => $.analytics.trend.empty)}
            </div>
          ) : (
            daily.map((row) => (
              <div key={row.date} className="flex min-w-0 flex-1 flex-col items-center gap-1">
                <div
                  className="w-full rounded-t bg-primary/25"
                  style={{ height: `${Math.max(6, (row.total_tokens / maxDailyTokens) * 100)}%` }}
                  title={`${row.date} · ${formatTokens(row.total_tokens)}`}
                />
                <span className="w-full truncate text-center text-[10px] text-muted-foreground">
                  {row.date.slice(5)}
                </span>
              </div>
            ))
          )}
        </div>
      </section>
      <section className="rounded-lg border bg-background p-3">
        <h2 className="text-sm font-medium">{t(($) => $.analytics.breakdown.title)}</h2>
        <div className="mt-3 grid grid-cols-2 gap-2 text-xs">
          <BreakdownItem label={t(($) => $.analytics.breakdown.input)} value={formatCost(breakdown.input)} />
          <BreakdownItem label={t(($) => $.analytics.breakdown.output)} value={formatCost(breakdown.output)} />
          <BreakdownItem label={t(($) => $.analytics.breakdown.cache_read)} value={formatCost(breakdown.cacheRead)} />
          <BreakdownItem label={t(($) => $.analytics.breakdown.cache_write)} value={formatCost(breakdown.cacheWrite)} />
        </div>
        {topSources.length > 0 && (
          <div className="mt-4">
            <div className="mb-2 text-xs font-medium text-muted-foreground">
              {t(($) => $.analytics.breakdown.top_sources)}
            </div>
            <div className="space-y-1">
              {topSources.map((source) => (
                <div key={`${source.source_type}:${source.source_id ?? ""}`} className="flex items-center justify-between gap-3 text-xs">
                  <span className="min-w-0 truncate">
                    {source.issue_identifier ?? source.source_title ?? source.source_type}
                  </span>
                  <span className="shrink-0 text-muted-foreground">{formatTokens(source.total_tokens)}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </section>
    </div>
  );
}

function BreakdownItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border bg-muted/20 px-2 py-1.5">
      <div className="text-muted-foreground">{label}</div>
      <div className="mt-0.5 font-medium">{value}</div>
    </div>
  );
}

function AnalyticsRunTable({
  data,
  sort,
  onSortChange,
  onPageChange,
}: {
  data: AgentAnalyticsResponse;
  sort: AgentAnalyticsSort;
  onSortChange: (sort: AgentAnalyticsSort) => void;
  onPageChange: (offset: number) => void;
}) {
  const { t } = useT("usage");
  const [expanded, setExpanded] = useState<string | null>(null);
  const rows = data.runs;
  const canPrevious = data.pagination.offset > 0;
  const nextOffset = data.pagination.offset + data.pagination.limit;
  const canNext = nextOffset < data.pagination.total;

  return (
    <section className="rounded-lg border bg-background">
      <div className="flex items-center justify-between gap-3 border-b px-3 py-2">
        <div>
          <h2 className="text-sm font-medium">{t(($) => $.analytics.table.title)}</h2>
          <p className="text-xs text-muted-foreground">
            {t(($) => $.analytics.table.caption, { count: data.pagination.total })}
          </p>
        </div>
      </div>
      <Table className="min-w-[1360px] table-fixed">
        <colgroup>
          <col style={{ width: 40 }} />
          <col style={{ width: 320 }} />
          <col style={{ width: 130 }} />
          <col style={{ width: 220 }} />
          <col style={{ width: 120 }} />
          <col style={{ width: 120 }} />
          <col style={{ width: 130 }} />
          <col style={{ width: 110 }} />
          <col style={{ width: 100 }} />
          <col style={{ width: 70 }} />
        </colgroup>
        <TableHeader>
          <TableRow>
            <TableHead className="w-8" />
            <TableHead>{t(($) => $.analytics.table.source)}</TableHead>
            <TableHead>{t(($) => $.analytics.table.status)}</TableHead>
            <TableHead>{t(($) => $.analytics.table.agent)}</TableHead>
            <TableHead>
              <SortButton
                active={sort === "duration_desc"}
                onClick={() => onSortChange("duration_desc")}
              >
                {t(($) => $.analytics.table.duration)}
              </SortButton>
            </TableHead>
            <TableHead>{t(($) => $.analytics.table.first_text)}</TableHead>
            <TableHead>
              <SortButton
                active={sort === "tokens_desc"}
                onClick={() => onSortChange("tokens_desc")}
              >
                {t(($) => $.analytics.table.tokens)}
              </SortButton>
            </TableHead>
            <TableHead>{t(($) => $.analytics.table.cost)}</TableHead>
            <TableHead>{t(($) => $.analytics.table.cache)}</TableHead>
            <TableHead>{t(($) => $.analytics.table.tools)}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <AnalyticsRunRows
              key={row.task_id}
              row={row}
              expanded={expanded === row.task_id}
              onToggle={() => setExpanded((current) => current === row.task_id ? null : row.task_id)}
            />
          ))}
        </TableBody>
      </Table>
      <div className="flex items-center justify-between border-t px-3 py-2 text-xs text-muted-foreground">
        <span>
          {t(($) => $.analytics.table.page, {
            start: data.pagination.total === 0 ? 0 : data.pagination.offset + 1,
            end: Math.min(data.pagination.offset + rows.length, data.pagination.total),
            total: data.pagination.total,
          })}
        </span>
        <div className="flex items-center gap-2">
          <Button size="xs" variant="outline" disabled={!canPrevious} onClick={() => onPageChange(Math.max(0, data.pagination.offset - data.pagination.limit))}>
            {t(($) => $.analytics.table.previous)}
          </Button>
          <Button size="xs" variant="outline" disabled={!canNext} onClick={() => onPageChange(nextOffset)}>
            {t(($) => $.analytics.table.next)}
          </Button>
        </div>
      </div>
    </section>
  );
}

function SortButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      className={cn("inline-flex items-center gap-1 hover:text-foreground", active && "text-foreground")}
      onClick={onClick}
    >
      {children}
      <ChevronDown className={cn("size-3 transition-transform", active && "rotate-180")} />
    </button>
  );
}

function AnalyticsRunRows({
  row,
  expanded,
  onToggle,
}: {
  row: AgentAnalyticsRunRow;
  expanded: boolean;
  onToggle: () => void;
}) {
  const { t } = useT("usage");
  const source = useRunSource(row);
  const cost = modelUsageCost(row.model_usage);
  const cacheRate = rate(row.cache_read_tokens, row.input_tokens + row.cache_read_tokens + row.cache_write_tokens);
  const toolCount = row.tracing.task_message_tool_use_count;
  const labels = {
    completed: t(($) => $.analytics.status.completed),
    failed: t(($) => $.analytics.status.failed),
    cancelled: t(($) => $.analytics.status.cancelled),
  };

  return (
    <>
      <TableRow>
        <TableCell>
          <Button size="icon-xs" variant="ghost" onClick={onToggle} aria-label={t(($) => $.analytics.table.expand)}>
            {expanded ? <ChevronDown /> : <ChevronRight />}
          </Button>
        </TableCell>
        <TableCell className="overflow-hidden">
          {source.href ? (
            <AppLink
              href={source.href}
              className="block min-w-0 max-w-full truncate text-foreground hover:underline"
              title={source.label}
            >
              {source.label}
            </AppLink>
          ) : (
            <span className="block min-w-0 max-w-full truncate" title={source.label}>
              {source.label}
            </span>
          )}
        </TableCell>
        <TableCell>
          <Badge variant={row.status === "failed" ? "destructive" : "outline"}>
            {statusLabel(row.status, labels)}
          </Badge>
        </TableCell>
        <TableCell className="overflow-hidden">
          <div className="block min-w-0 max-w-full truncate" title={row.agent_name || row.agent_id}>
            {row.agent_name || row.agent_id}
          </div>
          <div className="block min-w-0 max-w-full truncate text-xs text-muted-foreground">
            {row.model_usage[0]?.model ?? t(($) => $.analytics.table.unknown_model)}
          </div>
        </TableCell>
        <TableCell>{formatDuration(row.execution_ms || row.total_duration_ms)}</TableCell>
        <TableCell>{formatDuration(row.tracing.first_text_ms)}</TableCell>
        <TableCell>{formatTokens(row.total_tokens)}</TableCell>
        <TableCell>{formatCost(cost)}</TableCell>
        <TableCell>{formatPercent(cacheRate)}</TableCell>
        <TableCell>{toolCount}</TableCell>
      </TableRow>
      {expanded && (
        <TableRow>
          <TableCell colSpan={10} className="bg-muted/20 p-0">
            <RunDetails row={row} />
          </TableCell>
        </TableRow>
      )}
    </>
  );
}

function RunDetails({ row }: { row: AgentAnalyticsRunRow }) {
  const { t } = useT("usage");
  const breakdown = costBreakdown(row.model_usage);
  return (
    <div className="grid gap-3 p-3 text-xs md:grid-cols-3">
      <DetailGroup
        title={t(($) => $.analytics.details.latency)}
        rows={[
          [t(($) => $.analytics.details.queue), formatDuration(row.queue_ms)],
          [t(($) => $.analytics.details.startup), formatDuration(row.startup_ms)],
          [t(($) => $.analytics.details.execution), formatDuration(row.execution_ms)],
          [t(($) => $.analytics.details.first_event), formatDuration(row.tracing.first_event_ms)],
          [t(($) => $.analytics.details.first_tool), formatDuration(row.tracing.first_tool_use_ms)],
        ]}
      />
      <DetailGroup
        title={t(($) => $.analytics.details.context)}
        rows={[
          [t(($) => $.analytics.details.prompt), formatBytes(row.tracing.prompt_bytes)],
          [t(($) => $.analytics.details.system), formatBytes(row.tracing.system_prompt_bytes)],
          [t(($) => $.analytics.details.chat), formatBytes(row.tracing.chat_message_bytes)],
          [t(($) => $.analytics.details.skills), `${row.tracing.agent_skill_count} · ${formatBytes(row.tracing.agent_skill_bytes)}`],
          [t(($) => $.analytics.details.workflow), formatBytes(row.tracing.workflow_snapshot_bytes + row.tracing.workflow_step_snapshot_bytes)],
        ]}
      />
      <DetailGroup
        title={t(($) => $.analytics.details.output)}
        rows={[
          [t(($) => $.analytics.details.text), formatBytes(row.tracing.assistant_text_bytes)],
          [t(($) => $.analytics.details.thinking), formatBytes(row.tracing.thinking_bytes)],
          [t(($) => $.analytics.details.tool_input), formatBytes(row.tracing.tool_input_bytes)],
          [t(($) => $.analytics.details.tool_result), formatBytes(row.tracing.tool_result_bytes)],
          [t(($) => $.analytics.details.cost_segments), `${formatCost(breakdown.input)} / ${formatCost(breakdown.output)} / ${formatCost(breakdown.cacheRead)} / ${formatCost(breakdown.cacheWrite)}`],
        ]}
      />
    </div>
  );
}

function DetailGroup({ title, rows }: { title: string; rows: Array<[string, string]> }) {
  return (
    <div className="rounded-md border bg-background p-2">
      <div className="mb-2 font-medium">{title}</div>
      <div className="space-y-1">
        {rows.map(([label, value]) => (
          <div key={label} className="flex items-center justify-between gap-3">
            <span className="text-muted-foreground">{label}</span>
            <span className="text-right font-medium">{value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function AgentAnalyticsSkeleton() {
  return (
    <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-4">
        <div className="flex gap-2">
          <Skeleton className="h-7 w-40 rounded-lg" />
          <Skeleton className="h-7 w-48 rounded-lg" />
        </div>
        <div className="grid gap-3 md:grid-cols-3">
          <Skeleton className="h-28 rounded-lg" />
          <Skeleton className="h-28 rounded-lg" />
          <Skeleton className="h-28 rounded-lg" />
        </div>
        <div className="grid gap-3 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
          <Skeleton className="h-44 rounded-lg" />
          <Skeleton className="h-44 rounded-lg" />
        </div>
        <Skeleton className="h-80 rounded-lg" />
      </div>
    </div>
  );
}

function metricValues(summary: AgentAnalyticsSummary) {
  return {
    tokens: summary.total_tokens,
    cost: modelUsageCost(summary.model_usage),
    speedMs: summary.median_completion_ms,
  };
}

function modelUsageCost(rows: readonly AgentAnalyticsModelUsage[]) {
  return rows.reduce((sum, row) => sum + estimateCost(row), 0);
}

function costBreakdown(rows: readonly AgentAnalyticsModelUsage[]) {
  return rows.reduce(
    (sum, row) => {
      const next = estimateCostBreakdown(row);
      return {
        input: sum.input + next.input,
        output: sum.output + next.output,
        cacheRead: sum.cacheRead + next.cacheRead,
        cacheWrite: sum.cacheWrite + next.cacheWrite,
      };
    },
    { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
  );
}

function unpricedTokenTotal(rows: readonly AgentAnalyticsModelUsage[]) {
  return rows.reduce(
    (sum, row) => sum + (isModelPriced(row.model) ? 0 : row.total_tokens),
    0,
  );
}

function deltaText(current: number, previous: number) {
  if (!Number.isFinite(current) || !Number.isFinite(previous) || previous <= 0) return null;
  const change = (current - previous) / previous;
  if (Math.abs(change) < 0.005) return "0%";
  const sign = change > 0 ? "+" : "";
  return `${sign}${(change * 100).toFixed(0)}%`;
}

function dailyTokenBuckets(data: AgentAnalyticsResponse) {
  const buckets = new Map<string, { date: string; total_tokens: number }>();
  for (const row of data.daily) {
    const bucket = buckets.get(row.date) ?? { date: row.date, total_tokens: 0 };
    bucket.total_tokens += row.total_tokens;
    buckets.set(row.date, bucket);
  }
  return [...buckets.values()].sort((a, b) => a.date.localeCompare(b.date));
}

function rate(numerator: number, denominator: number) {
  if (denominator <= 0) return 0;
  return numerator / denominator;
}

function formatPercent(value: number) {
  if (!Number.isFinite(value)) return "0%";
  return `${Math.round(value * 100)}%`;
}

function formatCost(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "$0.00";
  if (value < 0.01) return `<$0.01`;
  return `$${value.toFixed(2)}`;
}

function formatDuration(ms: number) {
  if (!Number.isFinite(ms) || ms <= 0) return "—";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  const seconds = ms / 1000;
  if (seconds < 60) return `${seconds.toFixed(seconds < 10 ? 1 : 0)}s`;
  const minutes = Math.floor(seconds / 60);
  const remaining = Math.round(seconds % 60);
  return remaining > 0 ? `${minutes}m ${remaining}s` : `${minutes}m`;
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function useRunSource(row: AgentAnalyticsRunRow) {
  const wsPaths = useWorkspacePaths();
  if (row.source_type === "issue" && row.issue_id) {
    return {
      label: row.issue_identifier || row.issue_title || row.issue_id,
      href: wsPaths.issueDetail(row.issue_id),
    };
  }
  if (row.source_type === "chat" && row.chat_session_id) {
    return {
      label: row.chat_title || row.chat_session_id,
      href: wsPaths.chatSession(row.chat_session_id),
    };
  }
  return { label: row.task_id, href: null };
}

function statusLabel(
  status: string,
  labels: { completed: string; failed: string; cancelled: string },
) {
  switch (status) {
    case "completed":
      return labels.completed;
    case "failed":
      return labels.failed;
    case "cancelled":
      return labels.cancelled;
    default:
      return status;
  }
}
