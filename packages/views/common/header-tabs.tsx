"use client";

import type { CSSProperties } from "react";
import { cn } from "@multica/ui/lib/utils";

export interface HeaderTabItem<T extends string> {
  value: T;
  label: string;
  count?: number;
}

interface HeaderTabsProps<T extends string> {
  value: T;
  items: readonly HeaderTabItem<T>[];
  onValueChange: (value: T) => void;
  ariaLabel: string;
  className?: string;
}

export function HeaderTabs<T extends string>({
  value,
  items,
  onValueChange,
  ariaLabel,
  className,
}: HeaderTabsProps<T>) {
  const activeIndex = Math.max(0, items.findIndex((item) => item.value === value));
  const tabCount = Math.max(1, items.length);
  const gridStyle: CSSProperties = {
    gridTemplateColumns: `repeat(${tabCount}, minmax(0, 1fr))`,
  };
  const indicatorStyle: CSSProperties = {
    width: `calc((100% - 0.5rem) / ${tabCount})`,
    transform: `translateX(${activeIndex * 100}%)`,
  };

  return (
    <div
      role="tablist"
      aria-label={ariaLabel}
      className={cn(
        "relative grid h-9 shrink-0 rounded-xl border border-border/70 bg-muted/70 p-1 text-muted-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.55)]",
        className,
      )}
      style={gridStyle}
    >
      <span
        aria-hidden="true"
        className="pointer-events-none absolute bottom-1 left-1 top-1 rounded-lg bg-background shadow-sm ring-1 ring-border/70 transition-transform duration-200 ease-out motion-reduce:transition-none"
        style={indicatorStyle}
      />
      {items.map((item, itemIndex) => {
        const active = item.value === value;
        const selectAt = (nextIndex: number) => {
          const next = items[nextIndex];
          if (next && next.value !== value) onValueChange(next.value);
        };
        return (
          <button
            key={item.value}
            type="button"
            role="tab"
            aria-selected={active}
            tabIndex={active ? 0 : -1}
            onClick={() => {
              if (!active) onValueChange(item.value);
            }}
            onKeyDown={(event) => {
              if (event.key === "ArrowRight") {
                event.preventDefault();
                selectAt((itemIndex + 1) % tabCount);
              } else if (event.key === "ArrowLeft") {
                event.preventDefault();
                selectAt((itemIndex - 1 + tabCount) % tabCount);
              } else if (event.key === "Home") {
                event.preventDefault();
                selectAt(0);
              } else if (event.key === "End") {
                event.preventDefault();
                selectAt(tabCount - 1);
              }
            }}
            className={cn(
              "relative z-10 flex min-w-0 items-center justify-center gap-1.5 rounded-lg px-2.5 text-sm font-medium transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/45",
              active ? "text-foreground" : "text-muted-foreground hover:text-foreground",
            )}
          >
            <span className="truncate">{item.label}</span>
            {typeof item.count === "number" && item.count > 0 && (
              <span
                className={cn(
                  "shrink-0 rounded-full px-1.5 py-0.5 text-[0.65rem] leading-none transition-colors",
                  active ? "bg-muted text-muted-foreground" : "bg-background/70 text-muted-foreground",
                )}
              >
                {item.count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
