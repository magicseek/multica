import { render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

type TitleContent =
  | ""
  | {
      type: "doc";
      content: Array<{
        type: "paragraph";
        content?: Array<{ type: "text"; text: string }>;
      }>;
    };

function textFromContent(content: TitleContent): string {
  if (!content) return "";
  return content.content[0]?.content?.[0]?.text ?? "";
}

let currentText = "";
let isFocused = false;
let editor: {
  isFocused: boolean;
  commands: {
    setContent: ReturnType<typeof vi.fn>;
    focus: ReturnType<typeof vi.fn>;
    blur: ReturnType<typeof vi.fn>;
  };
  getText: ReturnType<typeof vi.fn>;
} | null = null;

vi.mock("../i18n", () => ({
  useT: () => ({
    t: (selector: unknown) =>
      typeof selector === "function" ? "Title" : String(selector ?? ""),
  }),
}));

vi.mock("@tiptap/react", () => ({
  useEditor: (options: { content: TitleContent }) => {
    if (!editor) {
      currentText = textFromContent(options.content);
      editor = {
        get isFocused() {
          return isFocused;
        },
        commands: {
          setContent: vi.fn((content: TitleContent) => {
            currentText = textFromContent(content);
          }),
          focus: vi.fn(),
          blur: vi.fn(),
        },
        getText: vi.fn(() => currentText),
      };
    }
    return editor;
  },
  EditorContent: () => <div data-testid="title-editor">{currentText}</div>,
}));

import { TitleEditor } from "./title-editor";

describe("TitleEditor", () => {
  beforeEach(() => {
    currentText = "";
    isFocused = false;
    editor = null;
  });

  it("syncs an async default value when the editor is not focused", async () => {
    const { rerender } = render(
      <TitleEditor defaultValue="" placeholder="Untitled" />,
    );

    rerender(
      <TitleEditor defaultValue="Project issue and output smoke" placeholder="Untitled" />,
    );

    await waitFor(() => {
      expect(editor?.commands.setContent).toHaveBeenCalledWith(
        {
          type: "doc",
          content: [
            {
              type: "paragraph",
              content: [
                { type: "text", text: "Project issue and output smoke" },
              ],
            },
          ],
        },
        { emitUpdate: false },
      );
    });
  });

  it("does not replace local edits while the editor is focused", () => {
    isFocused = true;
    const { rerender } = render(
      <TitleEditor defaultValue="Initial title" placeholder="Untitled" />,
    );
    currentText = "Draft title";
    editor?.commands.setContent.mockClear();

    rerender(
      <TitleEditor defaultValue="Remote title" placeholder="Untitled" />,
    );

    expect(editor?.commands.setContent).not.toHaveBeenCalled();
  });
});
