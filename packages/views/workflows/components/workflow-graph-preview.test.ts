import { createElement } from "react";
import { fireEvent, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { WorkflowStep } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import {
  buildWorkflowGraphLayout,
  getWorkflowGraphViewportTransform,
  WorkflowGraphPreview,
} from "./workflow-graph-preview";

describe("buildWorkflowGraphLayout", () => {
  it("builds dependency edges from structured workflow steps", () => {
    const steps: WorkflowStep[] = [
      { id: "context", title: "Read context", order: 1 },
      {
        id: "implement",
        title: "Implement",
        order: 2,
        depends_on: ["context"],
      },
      {
        id: "verify",
        title: "Verify",
        order: 3,
        depends_on: ["implement"],
      },
    ];

    const layout = buildWorkflowGraphLayout(steps);

    expect(layout.nodes.map((node) => node.id)).toEqual([
      "context",
      "implement",
      "verify",
    ]);
    expect(layout.edges.map((edge) => edge.id)).toEqual([
      "context:0->implement:1",
      "implement:1->verify:2",
    ]);
    expect(layout.nodes[0]?.x).toBeLessThan(60);
    expect(layout.nodes[1]?.y).toBeGreaterThan(layout.nodes[0]?.y ?? 0);
    expect(layout.nodes[2]?.y).toBeGreaterThan(layout.nodes[1]?.y ?? 0);
  });

  it("records missing dependencies without creating dangling edges", () => {
    const steps: WorkflowStep[] = [
      {
        id: "verify",
        title: "Verify",
        order: 1,
        depends_on: ["implement"],
      },
    ];

    const layout = buildWorkflowGraphLayout(steps);

    expect(layout.edges).toHaveLength(0);
    expect(layout.missingDependencyIds).toEqual(["implement"]);
    expect(layout.nodes[0]?.missingDependencyIds).toEqual(["implement"]);
  });

  it("scales graph content from viewport size changes", () => {
    const layout = { width: 280, height: 900 };

    const compact = getWorkflowGraphViewportTransform(layout, {
      width: 360,
      height: 420,
    });
    const expanded = getWorkflowGraphViewportTransform(layout, {
      width: 900,
      height: 820,
    });

    expect(compact.scale).toBeGreaterThan(0);
    expect(expanded.scale).toBeGreaterThan(compact.scale);
    expect(expanded.width).toBe(Math.ceil(layout.width * expanded.scale));
    expect(expanded.height).toBe(Math.ceil(layout.height * expanded.scale));
  });

  it("caps sparse graph enlargement so nodes remain visually stable", () => {
    const transform = getWorkflowGraphViewportTransform(
      { width: 280, height: 244 },
      { width: 1200, height: 800 },
    );

    expect(transform.scale).toBe(1.15);
  });

  it("renders localized graph labels and step cards", () => {
    const steps: WorkflowStep[] = [
      { id: "context", title: "Read context", order: 1 },
      {
        id: "verify",
        title: "Verify",
        order: 2,
        depends_on: ["context"],
      },
    ];

    renderWithI18n(createElement(WorkflowGraphPreview, { steps }));

    expect(screen.getByRole("heading", { name: "Graph preview" })).toBeVisible();
    expect(screen.getByText("Read context")).toBeVisible();
    expect(screen.getByText("Verify")).toBeVisible();
    expect(screen.getByText("1 deps")).toBeVisible();
  });

  it("exposes a graph expand action when controlled by the preview tab", () => {
    const onExpandedChange = vi.fn();

    renderWithI18n(
      createElement(WorkflowGraphPreview, {
        steps: [{ id: "context", title: "Read context", order: 1 }],
        expanded: false,
        onExpandedChange,
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Expand graph" }));

    expect(onExpandedChange).toHaveBeenCalledWith(true);
  });
});
