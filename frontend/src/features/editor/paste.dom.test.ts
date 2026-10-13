import { EditorState, Prec } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { afterEach, describe, expect, it } from "vitest";

import { pasteInput } from "./paste";

// The paste handler on its own (FH-142); the whole editor's pastes are
// MarkdownEditor.dom.test.tsx.
describe("pasteInput", () => {
  let view: EditorView | undefined;
  afterEach(() => view?.destroy());

  const paste = (html: string, text: string) => {
    const data = new DataTransfer();
    data.setData("text/html", html);
    data.setData("text/plain", text);
    const event = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(event, "clipboardData", { value: data });
    return event;
  };

  // On a Mac, CodeMirror's own keys take Ctrl+Shift+V (select a page
  // down), so a key handler would never see it: the key is observed.
  it.each([
    ["Ctrl", { ctrlKey: true }],
    ["Cmd", { metaKey: true }],
  ])(
    "notes %s with Shift and V even when a key binding takes it",
    (_name, mod) => {
      const taken = { run: () => true };
      view = new EditorView({
        parent: document.body,
        state: EditorState.create({
          extensions: [
            Prec.highest(
              keymap.of([
                { key: "Ctrl-Shift-v", ...taken },
                { key: "Meta-Shift-v", ...taken },
                // A shifted letter is matched as its capital too.
                { key: "Ctrl-V", ...taken },
                { key: "Meta-V", ...taken },
              ]),
            ),
            pasteInput(),
          ],
        }),
      });
      const key = new KeyboardEvent("keydown", {
        key: "V",
        code: "KeyV",
        shiftKey: true,
        ...mod,
        bubbles: true,
        cancelable: true,
      });
      view.contentDOM.dispatchEvent(key);
      // The binding took the key...
      expect(key.defaultPrevented).toBe(true);
      // ...and the paste after it is still plain text.
      view.contentDOM.dispatchEvent(paste("<p><b>Bold</b></p>", "Bold"));
      expect(view.state.doc.toString()).toBe("Bold");
    },
  );
});
