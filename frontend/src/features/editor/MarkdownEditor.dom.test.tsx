import { completionStatus } from "@codemirror/autocomplete";
import { EditorSelection } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";

import MarkdownEditor from "./MarkdownEditor";
import type { MarkdownEditorHandle } from "./MarkdownField";

const context = { project: "flashheart", ticket: "FH-16" };
const keys = new Set(["FH"]);

// Editor renders the editor over text it owns, reporting every change.
function Editor({
  initial,
  onChange = () => undefined,
  handle,
}: {
  initial: string;
  onChange?(value: string): void;
  handle?: React.Ref<MarkdownEditorHandle>;
}) {
  const [value, setValue] = useState(initial);
  return (
    <MarkdownEditor
      label="Description"
      value={value}
      onChange={(next) => {
        setValue(next);
        onChange(next);
      }}
      context={context}
      keys={keys}
      autoFocus={false}
      handle={handle}
    />
  );
}

// The slash menu and the selection toolbar need layout and real focus in
// a shadow root, which happy-dom lacks: frontend/e2e/editor.spec.mjs
// drives them in the browser (D42).

// mount renders the editor and finds its CodeMirror view in the shadow root.
async function mount(initial: string, onChange = vi.fn()) {
  const handle = createRef<MarkdownEditorHandle>();
  const result = render(
    <Editor initial={initial} onChange={onChange} handle={handle} />,
  );
  const host = result.container.querySelector<HTMLElement>("[data-autofocus]");
  const content = await waitFor(() => {
    const found = host?.shadowRoot?.querySelector<HTMLElement>(".cm-content");
    if (!found) throw new Error("the editor did not mount");
    return found;
  });
  const view = EditorView.findFromDOM(content);
  if (!view) throw new Error("no view");
  return { ...result, host, content, view, onChange, handle };
}

describe("MarkdownEditor", () => {
  it("is a textbox named for its section, holding the markdown as written", async () => {
    const { content, view } = await mount("Some **bold** text.\n");
    expect(content.getAttribute("role")).toBe("textbox");
    expect(content.getAttribute("aria-label")).toBe("Description");
    expect(content.getAttribute("aria-multiline")).toBe("true");
    expect(view.state.doc.toString()).toBe("Some **bold** text.\n");
  });

  it("ticks a task from its checkbox, changing one character", async () => {
    const { content, onChange } = await mount("Intro\n\n- [ ] Ship it\n");
    const box = content.querySelector<HTMLInputElement>("input.fh-task");
    expect(box?.getAttribute("aria-label")).toBe("Ship it");
    act(() => box?.click());
    expect(onChange).toHaveBeenLastCalledWith("Intro\n\n- [x] Ship it\n");
  });

  it("ticks the task on the cursor's line with Alt+Enter", async () => {
    const { view, onChange } = await mount("- [x] Done\n- [ ] Open");
    act(() => {
      view.focus();
      view.dispatch({ selection: EditorSelection.cursor(15) });
    });
    act(() => {
      view.contentDOM.dispatchEvent(
        new KeyboardEvent("keydown", {
          key: "Enter",
          altKey: true,
          bubbles: true,
        }),
      );
    });
    expect(onChange).toHaveBeenLastCalledWith("- [x] Done\n- [x] Open");
  });

  it("switches to markdown and back without changing the text", async () => {
    const user = userEvent.setup();
    const text = "### Plan\n\n- [ ] Ship it\n";
    const { content, view, onChange } = await mount(text);
    expect(content.querySelector("input.fh-task")).not.toBeNull();
    await user.click(screen.getByRole("radio", { name: "Markdown" }));
    expect(content.querySelector("input.fh-task")).toBeNull();
    expect(view.dom.classList.contains("fh-markdown")).toBe(true);
    expect(view.state.doc.toString()).toBe(text);
    await user.click(screen.getByRole("radio", { name: "Formatted" }));
    expect(content.querySelector("input.fh-task")).not.toBeNull();
    expect(view.state.doc.toString()).toBe(text);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("pastes the plain text of copied HTML, never its markup", async () => {
    const { view, content } = await mount("Start.");
    act(() => {
      view.focus();
      view.dispatch({ selection: EditorSelection.cursor(6) });
    });
    const data = new DataTransfer();
    data.setData(
      "text/html",
      '<script>alert(1)</script><a href="javascript:alert(1)">bad link</a> <img src="https://evil.example/x.png">',
    );
    data.setData("text/plain", "bad link");
    const paste = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(paste, "clipboardData", { value: data });
    act(() => {
      content.dispatchEvent(paste);
    });
    expect(view.state.doc.toString()).toBe("Start.bad link");
  });

  it("leaves a dropped file out of the text", async () => {
    const { view, content } = await mount("Start.");
    const data = new DataTransfer();
    data.items.add(new File(["secret"], "notes.txt", { type: "text/plain" }));
    const drop = new Event("drop", { bubbles: true, cancelable: true });
    Object.defineProperty(drop, "dataTransfer", { value: data });
    act(() => {
      content.dispatchEvent(drop);
    });
    expect(drop.defaultPrevented).toBe(true);
    expect(view.state.doc.toString()).toBe("Start.");
  });

  it("leaves Ctrl+Enter and Escape to the page when no menu is open", async () => {
    const { view } = await mount("Tex");
    // Just after typing, the menu's query is pending but nothing shows:
    // Escape is still the page's.
    act(() => {
      view.dispatch({
        changes: { from: 3, insert: "t" },
        selection: EditorSelection.cursor(4),
        userEvent: "input.type",
      });
    });
    expect(completionStatus(view.state)).toBe("pending");
    for (const init of [
      { key: "Enter", ctrlKey: true },
      { key: "Enter", metaKey: true },
      { key: "Escape" },
    ]) {
      const event = new KeyboardEvent("keydown", {
        ...init,
        bubbles: true,
        cancelable: true,
      });
      act(() => {
        view.focus();
        view.contentDOM.dispatchEvent(event);
      });
      expect(event.defaultPrevented, init.key).toBe(false);
    }
    expect(view.state.doc.toString()).toBe("Text");
  });

  it("inserts text at the cursor for others (FH-18)", async () => {
    const { view, handle, onChange } = await mount("See .");
    act(() => {
      view.dispatch({ selection: EditorSelection.cursor(4) });
      handle.current?.insert("![shot](files/shot.png)");
    });
    expect(onChange).toHaveBeenLastCalledWith("See ![shot](files/shot.png).");
  });
});
