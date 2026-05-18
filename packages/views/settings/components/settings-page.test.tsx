import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enSettings from "../../locales/en/settings.json";
import { SettingsPage } from "./settings-page";

const { navigation } = vi.hoisted(() => ({
  navigation: {
    pathname: "/acme/settings",
    searchParams: new URLSearchParams("tab=workflows"),
    replace: vi.fn(),
  },
}));

vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({ id: "ws-1", name: "Agent Force", slug: "acme" }),
}));

vi.mock("../../navigation", () => ({
  useNavigation: () => navigation,
}));

vi.mock("../../runtimes", () => ({ RuntimesPage: () => <div data-testid="runtimes-page" /> }));
vi.mock("../../workflows", () => ({ WorkflowsPage: () => <div data-testid="workflows-page" /> }));
vi.mock("../../skills", () => ({ SkillsPage: () => <div data-testid="skills-page" /> }));

const resources = {
  en: {
    common: enCommon,
    settings: enSettings,
  },
};

describe("SettingsPage", () => {
  it("renders configure pages in a full-height content shell instead of the narrow settings form shell", () => {
    render(
      <I18nProvider locale="en" resources={resources}>
        <SettingsPage />
      </I18nProvider>,
    );

    const shell = screen.getByTestId("workflows-page").closest("[data-settings-content-shell]");
    expect(shell).toHaveAttribute("data-settings-content-shell", "configure");
    expect(shell?.className).toContain("h-full");
    expect(shell?.className).not.toContain("max-w-6xl");
  });

  it("constrains configure content height so nested pages own their scrolling", () => {
    render(
      <I18nProvider locale="en" resources={resources}>
        <SettingsPage />
      </I18nProvider>,
    );

    const workflows = screen.getByTestId("workflows-page");
    const configureShell = workflows.closest("[data-settings-content-shell]");
    const contentRegion = configureShell?.parentElement;
    const workflowsTabPanel = workflows.closest("[data-slot='tabs-content']");

    expect(contentRegion?.className).toContain("min-h-0");
    expect(configureShell?.className).toContain("overflow-hidden");
    expect(workflowsTabPanel?.className).toContain("overflow-hidden");
    expect(workflowsTabPanel?.className.split(/\s+/)).toContain("flex");
  });
});
