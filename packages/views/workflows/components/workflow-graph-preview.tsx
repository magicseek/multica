"use client";

import { useEffect, useRef, useState } from "react";
import type { WorkflowStep } from "@multica/core/types";
import { Badge } from "@multica/ui/components/ui/badge";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { Maximize2, Minimize2 } from "lucide-react";
import { useT } from "../../i18n";

const NODE_WIDTH = 224;
const NODE_HEIGHT = 96;
const PADDING_X = 28;
const PADDING_Y = 28;
const GAP_X = 72;
const GAP_Y = 92;
const VIEWPORT_PADDING = 48;
const MIN_OVERVIEW_SCALE = 0.75;
const MAX_FIT_SCALE = 1.15;

interface ViewportSize {
  width: number;
  height: number;
}

export interface WorkflowGraphViewportTransform {
  scale: number;
  width: number;
  height: number;
}

export interface WorkflowGraphNode {
  key: string;
  id: string;
  title: string;
  description: string;
  order: number;
  required: boolean;
  dependencyIds: string[];
  missingDependencyIds: string[];
  x: number;
  y: number;
}

export interface WorkflowGraphEdge {
  id: string;
  sourceKey: string;
  targetKey: string;
  sourceX: number;
  sourceY: number;
  targetX: number;
  targetY: number;
}

export interface WorkflowGraphLayout {
  nodes: WorkflowGraphNode[];
  edges: WorkflowGraphEdge[];
  width: number;
  height: number;
  missingDependencyIds: string[];
}

interface NormalizedStep {
  key: string;
  id: string;
  title: string;
  description: string;
  order: number;
  required: boolean;
  dependencyIds: string[];
}

function normalizeGraphSteps(steps: WorkflowStep[]): NormalizedStep[] {
  return steps.map((step, index) => {
    const id = (step.id ?? "").trim() || `step-${index + 1}`;
    const title = (step.title ?? "").trim() || step.name?.trim() || id;
    return {
      key: `${id}:${index}`,
      id,
      title,
      description:
        step.description?.trim() || step.body_template?.trim() || "",
      order: step.order || index + 1,
      required: step.required !== false,
      dependencyIds: (step.depends_on ?? [])
        .map((dependencyId) => dependencyId.trim())
        .filter(Boolean),
    };
  });
}

function depthForStep(
  step: NormalizedStep,
  firstStepById: Map<string, NormalizedStep>,
  cache: Map<string, number>,
  visiting: Set<string>,
): number {
  if (cache.has(step.key)) return cache.get(step.key) ?? 0;
  if (visiting.has(step.key)) return 0;

  visiting.add(step.key);
  const dependencyDepths = step.dependencyIds
    .map((dependencyId) => firstStepById.get(dependencyId))
    .filter((dependency): dependency is NormalizedStep => Boolean(dependency))
    .map((dependency) =>
      depthForStep(dependency, firstStepById, cache, new Set(visiting)) + 1,
    );
  const depth = dependencyDepths.length > 0 ? Math.max(...dependencyDepths) : 0;
  cache.set(step.key, depth);
  return depth;
}

export function buildWorkflowGraphLayout(
  steps: WorkflowStep[],
): WorkflowGraphLayout {
  const normalized = normalizeGraphSteps(steps).sort(
    (left, right) => left.order - right.order,
  );
  const firstStepById = new Map<string, NormalizedStep>();
  normalized.forEach((step) => {
    if (!firstStepById.has(step.id)) {
      firstStepById.set(step.id, step);
    }
  });

  const depthCache = new Map<string, number>();
  const layers = new Map<number, NormalizedStep[]>();
  normalized.forEach((step) => {
    const depth = depthForStep(step, firstStepById, depthCache, new Set());
    const layer = layers.get(depth) ?? [];
    layer.push(step);
    layers.set(depth, layer);
  });

  const maxLayerSize = Math.max(
    1,
    ...Array.from(layers.values()).map((layer) => layer.length),
  );
  const maxDepth = Math.max(0, ...Array.from(layers.keys()));
  const width =
    PADDING_X * 2 + maxLayerSize * NODE_WIDTH + (maxLayerSize - 1) * GAP_X;
  const height =
    PADDING_Y * 2 + (maxDepth + 1) * NODE_HEIGHT + maxDepth * GAP_Y;
  const nodeByKey = new Map<string, WorkflowGraphNode>();

  Array.from(layers.entries())
    .sort(([left], [right]) => left - right)
    .forEach(([depth, layer]) => {
      layer.forEach((step, index) => {
        const missingDependencyIds = step.dependencyIds.filter(
          (dependencyId) => !firstStepById.has(dependencyId),
        );
        nodeByKey.set(step.key, {
          ...step,
          missingDependencyIds,
          x: PADDING_X + index * (NODE_WIDTH + GAP_X),
          y: PADDING_Y + depth * (NODE_HEIGHT + GAP_Y),
        });
      });
    });

  const nodes = normalized
    .map((step) => nodeByKey.get(step.key))
    .filter((node): node is WorkflowGraphNode => Boolean(node));
  const edges: WorkflowGraphEdge[] = [];
  nodes.forEach((target) => {
    target.dependencyIds.forEach((dependencyId) => {
      const sourceStep = firstStepById.get(dependencyId);
      const source = sourceStep ? nodeByKey.get(sourceStep.key) : undefined;
      if (!source) return;
      edges.push({
        id: `${source.key}->${target.key}`,
        sourceKey: source.key,
        targetKey: target.key,
        sourceX: source.x + NODE_WIDTH / 2,
        sourceY: source.y + NODE_HEIGHT,
        targetX: target.x + NODE_WIDTH / 2,
        targetY: target.y,
      });
    });
  });

  return {
    nodes,
    edges,
    width,
    height,
    missingDependencyIds: Array.from(
      new Set(nodes.flatMap((node) => node.missingDependencyIds)),
    ),
  };
}

function edgePath(edge: WorkflowGraphEdge): string {
  const midY = edge.sourceY + (edge.targetY - edge.sourceY) / 2;
  return [
    `M ${edge.sourceX} ${edge.sourceY}`,
    `C ${edge.sourceX} ${midY}, ${edge.targetX} ${midY}, ${edge.targetX} ${edge.targetY}`,
  ].join(" ");
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export function getWorkflowGraphViewportTransform(
  layout: Pick<WorkflowGraphLayout, "width" | "height">,
  viewport: ViewportSize,
): WorkflowGraphViewportTransform {
  if (
    layout.width <= 0 ||
    layout.height <= 0 ||
    viewport.width <= 0 ||
    viewport.height <= 0
  ) {
    return {
      scale: 1,
      width: layout.width,
      height: layout.height,
    };
  }

  const availableWidth = Math.max(1, viewport.width - VIEWPORT_PADDING);
  const availableHeight = Math.max(1, viewport.height - VIEWPORT_PADDING);
  const fitScale = Math.min(
    availableWidth / layout.width,
    availableHeight / layout.height,
  );
  const scale = clamp(fitScale, MIN_OVERVIEW_SCALE, MAX_FIT_SCALE);

  return {
    scale,
    width: Math.ceil(layout.width * scale),
    height: Math.ceil(layout.height * scale),
  };
}

export function WorkflowGraphPreview({
  steps,
  expanded = false,
  onExpandedChange,
  className,
}: {
  steps: WorkflowStep[];
  expanded?: boolean;
  onExpandedChange?: (expanded: boolean) => void;
  className?: string;
}) {
  const { t } = useT("workflows");
  const layout = buildWorkflowGraphLayout(steps);
  const viewportRef = useRef<HTMLDivElement>(null);
  const [viewportSize, setViewportSize] = useState<ViewportSize>({
    width: 0,
    height: 0,
  });
  const transform = getWorkflowGraphViewportTransform(layout, viewportSize);
  const expandLabel = expanded
    ? t(($) => $.graph.collapse)
    : t(($) => $.graph.expand);

  useEffect(() => {
    const element = viewportRef.current;
    if (!element) return undefined;

    const updateSize = () => {
      setViewportSize({
        width: element.clientWidth,
        height: element.clientHeight,
      });
    };

    updateSize();

    if (typeof ResizeObserver === "undefined") {
      window.addEventListener("resize", updateSize);
      return () => window.removeEventListener("resize", updateSize);
    }

    const observer = new ResizeObserver(updateSize);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    const element = viewportRef.current;
    if (!element || layout.nodes.length === 0) return undefined;

    const frame = window.requestAnimationFrame(() => {
      element.scrollLeft = Math.max(
        0,
        (element.scrollWidth - element.clientWidth) / 2,
      );
      element.scrollTop = Math.max(
        0,
        (element.scrollHeight - element.clientHeight) / 2,
      );
    });

    return () => window.cancelAnimationFrame(frame);
  }, [
    expanded,
    layout.height,
    layout.nodes.length,
    layout.width,
    transform.height,
    transform.scale,
    transform.width,
    viewportSize.height,
    viewportSize.width,
  ]);

  return (
    <div
      className={cn(
        "flex min-h-[420px] flex-col rounded-md border bg-background",
        className,
      )}
    >
      <div className="flex h-10 shrink-0 items-center justify-between border-b px-3">
        <h2 className="text-xs font-medium">{t(($) => $.graph.title)}</h2>
        <div className="flex items-center gap-2">
          {layout.nodes.length > 0 && (
            <span className="font-mono text-xs tabular-nums text-muted-foreground">
              {layout.nodes.length}
            </span>
          )}
          {onExpandedChange && (
            <Button
              type="button"
              size="icon"
              variant="ghost"
              className="h-7 w-7"
              onClick={() => onExpandedChange(!expanded)}
              aria-label={expandLabel}
              title={expandLabel}
            >
              {expanded ? (
                <Minimize2 className="h-3.5 w-3.5" />
              ) : (
                <Maximize2 className="h-3.5 w-3.5" />
              )}
            </Button>
          )}
        </div>
      </div>

      {layout.nodes.length === 0 ? (
        <div className="flex flex-1 items-center justify-center px-4 text-center text-sm text-muted-foreground">
          {t(($) => $.graph.empty)}
        </div>
      ) : (
        <div
          ref={viewportRef}
          className="min-h-0 flex-1 overflow-auto bg-muted/5"
        >
          <div
            className="flex items-center justify-center p-6"
            style={{
              minWidth: Math.max(viewportSize.width, transform.width + 48),
              minHeight: Math.max(viewportSize.height, transform.height + 48),
            }}
          >
            <div
              className="relative shrink-0"
              style={{
                width: transform.width,
                height: transform.height,
              }}
            >
              <svg
                aria-hidden="true"
                className="pointer-events-none absolute inset-0"
                height={transform.height}
                preserveAspectRatio="none"
                viewBox={`0 0 ${layout.width} ${layout.height}`}
                width={transform.width}
              >
                <defs>
                  <marker
                    id="workflow-graph-arrow"
                    markerHeight="8"
                    markerWidth="8"
                    orient="auto"
                    refX="7"
                    refY="4"
                  >
                    <path d="M 0 0 L 8 4 L 0 8 z" className="fill-border" />
                  </marker>
                </defs>
                {layout.edges.map((edge) => (
                  <path
                    key={edge.id}
                    d={edgePath(edge)}
                    className="fill-none stroke-border"
                    markerEnd="url(#workflow-graph-arrow)"
                    strokeWidth="1.5"
                  />
                ))}
              </svg>

              {layout.nodes.map((node) => (
                <div
                  key={node.key}
                  className="absolute"
                  style={{
                    left: node.x * transform.scale,
                    top: node.y * transform.scale,
                    width: NODE_WIDTH * transform.scale,
                    minHeight: NODE_HEIGHT * transform.scale,
                  }}
                >
                  <div
                    className={cn(
                      "flex min-h-24 flex-col rounded-md border bg-background px-3 py-2 shadow-sm",
                      node.missingDependencyIds.length > 0 &&
                        "border-destructive/60 bg-destructive/5",
                    )}
                    style={{
                      width: NODE_WIDTH,
                      minHeight: NODE_HEIGHT,
                      transform: `scale(${transform.scale})`,
                      transformOrigin: "top left",
                    }}
                  >
                    <div className="flex items-start gap-2">
                      <span className="flex h-5 min-w-5 items-center justify-center rounded-md bg-muted font-mono text-[10px] text-muted-foreground">
                        {node.order}
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-xs font-medium">
                          {node.title}
                        </div>
                        <div className="truncate font-mono text-[10px] text-muted-foreground">
                          {node.id}
                        </div>
                      </div>
                    </div>
                    {node.description && (
                      <p className="mt-2 line-clamp-2 text-xs text-muted-foreground">
                        {node.description}
                      </p>
                    )}
                    <div className="mt-auto flex flex-wrap gap-1 pt-2">
                      <Badge
                        variant="outline"
                        className="h-4 rounded-md px-1 text-[10px]"
                      >
                        {node.required
                          ? t(($) => $.graph.required)
                          : t(($) => $.graph.optional)}
                      </Badge>
                      <Badge
                        variant="outline"
                        className="h-4 rounded-md px-1 text-[10px]"
                      >
                        {node.dependencyIds.length > 0
                          ? t(($) => $.graph.dependency_count, {
                              count: node.dependencyIds.length,
                            })
                          : t(($) => $.graph.no_dependencies)}
                      </Badge>
                    </div>
                    {node.missingDependencyIds.length > 0 && (
                      <div className="mt-1 truncate text-[10px] text-destructive">
                        {t(($) => $.graph.missing_dependency, {
                          id: node.missingDependencyIds.join(", "),
                        })}
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
