import {
  EditorSelection,
  type EditorState,
  type Extension,
} from "@codemirror/state";
import { EditorView } from "@codemirror/view";

import { clipboardFiles } from "../../model/uploads";
import { htmlToMarkdown } from "./htmlToMarkdown";
import type { AttachHandler } from "./uploads";

// The editor's one paste handler, in order of priority:
// 1. files with no text (a screenshot) are attached (FH-18, onFiles);
// 2. after Ctrl or Cmd with Shift and V, the plain text is pasted;
// 3. copied HTML that holds what markdown keeps is pasted as markdown
//    (FH-142, htmlToMarkdown);
// 4. anything else is left to CodeMirror, which pastes the plain text.
//
// The paste event does not say whether Shift was held, and browsers
// differ in whether their paste-as-plain-text leaves the HTML out
// (Chromium does), so the key itself is noted as it is pressed.

// PLAIN_KEY_MS is how soon after the key its paste must come.
const PLAIN_KEY_MS = 1000;

// plainKeyAt is when each editor last saw the plain-text paste key.
const plainKeyAt = new WeakMap<EditorView, number>();

// isPlainPasteKey is Ctrl or Cmd with Shift and V (with Alt too, Safari's
// Paste and Match Style).
function isPlainPasteKey(event: KeyboardEvent): boolean {
  return (
    (event.ctrlKey || event.metaKey) &&
    event.shiftKey &&
    (event.code === "KeyV" || event.key.toLowerCase() === "v")
  );
}

export function pasteInput(onFiles?: AttachHandler): Extension {
  return [
    // Observed, not handled: a key binding that takes the key (on a Mac,
    // CodeMirror's Ctrl+Shift+V selects a page down) must not hide it.
    EditorView.domEventObservers({
      keydown(event, view) {
        if (isPlainPasteKey(event)) plainKeyAt.set(view, Date.now());
        else plainKeyAt.delete(view);
      },
    }),
    pasteHandler(onFiles),
  ];
}

function pasteHandler(onFiles?: AttachHandler): Extension {
  return EditorView.domEventHandlers({
    paste(event, view) {
      const keyAt = plainKeyAt.get(view);
      plainKeyAt.delete(view);
      const data = event.clipboardData;
      const files = onFiles ? clipboardFiles(data) : [];
      if (onFiles && files.length) {
        event.preventDefault();
        const { main } = view.state.selection;
        if (!main.empty)
          view.dispatch({
            ...view.state.replaceSelection(""),
            userEvent: "delete",
          });
        onFiles(view, files, view.state.selection.main.head, true);
        return true;
      }
      if (keyAt !== undefined && Date.now() - keyAt < PLAIN_KEY_MS)
        return false;
      const html = data?.getData("text/html");
      const markdown = html ? htmlToMarkdown(html) : null;
      if (markdown === null) return false;
      event.preventDefault();
      view.dispatch(
        insertMarkdown(view.state, markdown),
        // Named as CodeMirror's own paste is.
        { userEvent: "input.paste", scrollIntoView: true },
      );
      return true;
    },
  });
}

// BLOCK_START is markdown that must start its own line: a heading, a list
// item, a quote, a fence, a table or a rule.
const BLOCK_START = /^(#{1,6} |- |\d+\. |> ?|`{3,}|\| |---$)/;

// insertMarkdown replaces each selection with pasted markdown; markdown
// that holds blocks goes on lines of its own, a blank line from the text
// around it, so a pasted heading never lands mid-sentence.
function insertMarkdown(state: EditorState, markdown: string) {
  const blocks = markdown.includes("\n\n") || BLOCK_START.test(markdown);
  return state.changeByRange(({ from, to }) => {
    const before = state.sliceDoc(state.doc.lineAt(from).from, from);
    const after = state.sliceDoc(to, state.doc.lineAt(to).to);
    const lead = blocks && before.trim() ? "\n\n" : "";
    const trail = blocks && after.trim() ? "\n\n" : "";
    return {
      changes: { from, to, insert: lead + markdown + trail },
      // The cursor ends after the pasted text, as a typed paste's does.
      range: EditorSelection.cursor(from + lead.length + markdown.length),
    };
  });
}
