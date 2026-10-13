import { completionStatus } from "@codemirror/autocomplete";
import { EditorSelection } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import type { Upload } from "../../model/uploads";
import MarkdownEditor from "./MarkdownEditor";

const context = { project: "flashheart", ticket: "FH-16" };
const keys = new Set(["FH"]);

// Editor renders the editor over text it owns, reporting every change.
function Editor({
  initial,
  onChange = () => undefined,
  upload,
}: {
  initial: string;
  onChange?(value: string): void;
  upload?: Upload;
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
      upload={upload}
    />
  );
}

// The slash menu and the selection toolbar need layout and real focus in
// a shadow root, which happy-dom lacks: frontend/e2e/editor.spec.mjs
// drives them in the browser (D42).

// mount renders the editor and finds its CodeMirror view in the shadow root.
async function mount(initial: string, onChange = vi.fn(), upload?: Upload) {
  const result = render(
    <Editor initial={initial} onChange={onChange} upload={upload} />,
  );
  const host = result.container.querySelector<HTMLElement>("[data-autofocus]");
  const content = await waitFor(() => {
    const found = host?.shadowRoot?.querySelector<HTMLElement>(".cm-content");
    if (!found) throw new Error("the editor did not mount");
    return found;
  });
  const view = EditorView.findFromDOM(content);
  if (!view) throw new Error("no view");
  return { ...result, host, content, view, onChange };
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
});

// Files pasted, dropped or chosen (FH-18): a placeholder while each
// uploads, then its reference at the cursor; a refusal removes it and
// says why. The text saved never holds a placeholder.
describe("MarkdownEditor with files", () => {
  // held is an upload whose results the test settles, one per file.
  function held() {
    const calls: {
      name: string;
      kind?: string;
      resolve(file: string, image: boolean): void;
      reject(error: Error): void;
    }[] = [];
    const upload: Upload = (_file, name, kind) =>
      new Promise((resolve, reject) => {
        calls.push({
          name,
          kind,
          resolve: (file, image) =>
            resolve({ file, kind: kind ?? "other", image, reused: false }),
          reject,
        });
      });
    return { calls, upload };
  }
  const fileEvent = (type: "paste" | "drop", files: File[], text = "") => {
    const data = new DataTransfer();
    if (text) data.setData("text/plain", text);
    for (const file of files) data.items.add(file);
    const event = new Event(type, { bubbles: true, cancelable: true });
    Object.defineProperty(
      event,
      type === "paste" ? "clipboardData" : "dataTransfer",
      { value: data },
    );
    return event;
  };
  const png = (name: string) =>
    new File(["\x89PNG"], name, { type: "image/png" });
  const placeholders = (content: HTMLElement) =>
    Array.from(content.querySelectorAll<HTMLElement>(".fh-upload")).map(
      (element) => element.textContent,
    );

  it("pastes a screenshot as an image at the cursor, its placeholder showing meanwhile", async () => {
    const { calls, upload } = held();
    const onChange = vi.fn();
    const { view, content } = await mount("See .", onChange, upload);
    act(() => view.dispatch({ selection: EditorSelection.cursor(4) }));
    const paste = fileEvent("paste", [png("image.png")]);
    act(() => {
      content.dispatchEvent(paste);
    });
    expect(paste.defaultPrevented).toBe(true);
    expect(calls.map(({ name, kind }) => [name, kind])).toEqual([
      ["screenshot.png", "screenshot"],
    ]);
    expect(placeholders(content)).toEqual(["Uploading screenshot.png…"]);
    expect(screen.getByRole("status").textContent).toBe("Uploading 1 file…");
    expect(view.state.doc.toString()).toBe("See .");
    expect(onChange).not.toHaveBeenCalled();

    await act(async () =>
      calls[0]?.resolve("20261013T1432-screenshot.png", true),
    );
    const text = "See ![screenshot](files/20261013T1432-screenshot.png).";
    await waitFor(() => expect(view.state.doc.toString()).toBe(text));
    expect(onChange).toHaveBeenLastCalledWith(text);
    expect(placeholders(content)).toEqual([]);
    expect(view.state.selection.main.head).toBe(text.length - 1);
    expect(screen.getByRole("status").textContent).toBe(
      "Attached screenshot.png.",
    );
  });

  it("pastes copied text as text even when an image comes with it", async () => {
    const { calls, upload } = held();
    const { view, content } = await mount("", vi.fn(), upload);
    act(() => {
      content.dispatchEvent(fileEvent("paste", [png("image.png")], "a1 b1"));
    });
    expect(calls).toEqual([]);
    expect(view.state.doc.toString()).toBe("a1 b1");
  });

  it("drops several files in order, images as images and others as links", async () => {
    const { calls, upload } = held();
    const { view, content } = await mount("Intro\n", vi.fn(), upload);
    act(() => view.dispatch({ selection: EditorSelection.cursor(6) }));
    const drop = fileEvent("drop", [
      new File(["%PDF-"], "spec.pdf", { type: "application/pdf" }),
      png("before.png"),
      png("after.png"),
    ]);
    act(() => {
      content.dispatchEvent(drop);
    });
    expect(drop.defaultPrevented).toBe(true);
    expect(placeholders(content)).toEqual([
      "Uploading spec.pdf…",
      "Uploading before.png…",
      "Uploading after.png…",
    ]);
    // One at a time: the next is sent when the first is stored.
    expect(calls).toHaveLength(1);
    await act(async () => calls[0]?.resolve("x-spec.pdf", false));
    expect(placeholders(content)).toEqual([
      "Uploading before.png…",
      "Uploading after.png…",
    ]);
    await waitFor(() => expect(calls).toHaveLength(2));
    await act(async () => calls[1]?.resolve("x-before.png", true));
    await waitFor(() => expect(calls).toHaveLength(3));
    await act(async () => calls[2]?.resolve("x-after.png", true));
    await waitFor(() =>
      expect(view.state.doc.toString()).toBe(
        "Intro\n[spec.pdf](files/x-spec.pdf)\n![before](files/x-before.png)\n![after](files/x-after.png)",
      ),
    );
    expect(placeholders(content)).toEqual([]);
    expect(screen.getByRole("status").textContent).toBe("Attached 3 files.");
  });

  it("removes a refused file's placeholder and says why, keeping the others", async () => {
    const { calls, upload } = held();
    const onChange = vi.fn();
    const { view, content } = await mount("", onChange, upload);
    const big = png("huge.png");
    Object.defineProperty(big, "size", { value: 21 * 1024 * 1024 });
    act(() => {
      content.dispatchEvent(
        fileEvent("drop", [png("drawing.png"), big, png("ok.png")]),
      );
    });
    await act(async () =>
      calls[0]?.reject(
        Object.assign(
          new Error("drawing.png was not attached because it is an SVG image."),
          { code: "type_not_allowed" },
        ),
      ),
    );
    // The oversized file is never sent.
    await waitFor(() =>
      expect(calls.map((call) => call.name)).toEqual(["drawing.png", "ok.png"]),
    );
    await act(async () => calls[1]?.resolve("x-ok.png", true));
    await waitFor(() =>
      expect(view.state.doc.toString()).toBe("![ok](files/x-ok.png)"),
    );
    expect(placeholders(content)).toEqual([]);
    const alert = screen.getByRole("alert");
    expect(alert.textContent).toContain(
      "drawing.png was not attached because it is an SVG image.",
    );
    expect(alert.textContent).toContain(
      "huge.png is larger than 20 MB, the limit for an attachment.",
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(screen.queryByRole("alert")).toBeNull();
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("leaves nothing behind when every file is refused", async () => {
    const { calls, upload } = held();
    const onChange = vi.fn();
    const { view, content } = await mount("Keep.", onChange, upload);
    act(() => {
      content.dispatchEvent(fileEvent("drop", [png("a.png")]));
    });
    await act(async () => calls[0]?.reject(new Error("Network down")));
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain(
        "a.png was not attached: Network down",
      ),
    );
    expect(placeholders(content)).toEqual([]);
    expect(view.state.doc.toString()).toBe("Keep.");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("attaches chosen files at the cursor from the Attach files button", async () => {
    const { calls, upload } = held();
    const { view, container } = await mount("A B", vi.fn(), upload);
    act(() => view.dispatch({ selection: EditorSelection.cursor(2) }));
    const user = userEvent.setup();
    const input =
      container.querySelector<HTMLInputElement>('input[type="file"]');
    if (!input) throw new Error("no file input");
    const pick = vi.spyOn(input, "click");
    await user.click(screen.getByRole("button", { name: "Attach files" }));
    expect(pick).toHaveBeenCalled();
    await user.upload(
      input,
      new File(["log"], "build.log", { type: "text/plain" }),
    );
    expect(calls.map((call) => [call.name, call.kind])).toEqual([
      ["build.log", undefined],
    ]);
    await act(async () => calls[0]?.resolve("x-build.log", false));
    await waitFor(() =>
      expect(view.state.doc.toString()).toBe(
        "A [build.log](files/x-build.log)B",
      ),
    );
  });

  it("offers no Attach files without an upload, and keeps dropped files out", async () => {
    const { view, content } = await mount("Start.");
    expect(screen.queryByRole("button", { name: "Attach files" })).toBeNull();
    const drop = fileEvent("drop", [png("a.png")]);
    act(() => {
      content.dispatchEvent(drop);
    });
    expect(drop.defaultPrevented).toBe(true);
    expect(view.state.doc.toString()).toBe("Start.");
  });
});
