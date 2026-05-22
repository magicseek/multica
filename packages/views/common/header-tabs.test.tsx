import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { HeaderTabs } from "./header-tabs";

describe("HeaderTabs", () => {
  it("keeps tabs equal width and moves one active indicator", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    const items = [
      { value: "chat", label: "Chat" },
      { value: "issues", label: "Issues", count: 2 },
      { value: "outputs", label: "Outputs", count: 1 },
      { value: "analytics", label: "Analytics" },
    ] as const;

    const { container, rerender } = render(
      <HeaderTabs
        ariaLabel="Sections"
        value="chat"
        items={items}
        onValueChange={onValueChange}
      />,
    );

    const tablist = screen.getByRole("tablist", { name: "Sections" });
    expect(tablist.getAttribute("style")).toContain("repeat(4, minmax(0, 1fr))");

    const indicator = container.querySelector("[aria-hidden='true']");
    expect(indicator).toHaveStyle("transform: translateX(0%)");

    await user.click(screen.getByRole("tab", { name: /outputs/i }));
    expect(onValueChange).toHaveBeenCalledWith("outputs");

    rerender(
      <HeaderTabs
        ariaLabel="Sections"
        value="outputs"
        items={items}
        onValueChange={onValueChange}
      />,
    );
    expect(container.querySelector("[aria-hidden='true']")).toHaveStyle("transform: translateX(200%)");
  });

  it("supports keyboard navigation across tabs", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    const items = [
      { value: "issues", label: "Issues" },
      { value: "chats", label: "Chats" },
      { value: "analytics", label: "Analytics" },
    ] as const;

    render(
      <HeaderTabs
        ariaLabel="Project sections"
        value="issues"
        items={items}
        onValueChange={onValueChange}
      />,
    );

    screen.getByRole("tab", { name: "Issues" }).focus();
    await user.keyboard("{ArrowRight}");
    expect(onValueChange).toHaveBeenLastCalledWith("chats");

    await user.keyboard("{End}");
    expect(onValueChange).toHaveBeenLastCalledWith("analytics");

    await user.keyboard("{ArrowLeft}");
    expect(onValueChange).toHaveBeenLastCalledWith("analytics");
  });
});
