import type {
  Completion,
  CompletionContext,
  CompletionResult,
} from "@codemirror/autocomplete";
import {
  EditorSelection,
  type EditorState,
  type TransactionSpec,
} from "@codemirror/state";

import { inLinkOrCode } from "./links";

// The slash menu (FH-16): typing / at the start of a line or after a
// space offers the blocks a ticket is written with, and inserts each as
// markdown. Built on @codemirror/autocomplete, which owns its listbox,
// arrow keys, Enter and Escape.

export interface SlashItem {
  label: string;
  // detail is the markdown the item writes, shown beside its label.
  detail: string;
  // line items mark the line (after its indentation, replacing a list,
  // quote or heading mark); block items go on a line of their own; inline
  // items go at the cursor. text holds ‸ where the cursor goes.
  kind: "line" | "block" | "inline";
  text: string;
}

// Sections are the file's level-2 headings, so the menu offers level 3
// and 4: a ## heading in a section would start a new section.
export const slashItems: SlashItem[] = [
  { label: "Heading", detail: "###", kind: "line", text: "### " },
  { label: "Subheading", detail: "####", kind: "line", text: "#### " },
  { label: "Bulleted list", detail: "-", kind: "line", text: "- " },
  { label: "Numbered list", detail: "1.", kind: "line", text: "1. " },
  { label: "Task list", detail: "- [ ]", kind: "line", text: "- [ ] " },
  { label: "Quote", detail: ">", kind: "line", text: "> " },
  { label: "Code block", detail: "```", kind: "block", text: "```\n‸\n```" },
  {
    label: "Table",
    detail: "| |",
    kind: "block",
    text: "| Column | Column |\n| --- | --- |\n| ‸ |  |",
  },
  { label: "Divider", detail: "---", kind: "block", text: "---\n‸" },
  { label: "Link", detail: "[]()", kind: "inline", text: "[‸]()" },
];

const lineMark = /^(\s*)(?:#{1,6} |[-*+] \[[ xX]\] |[-*+] |\d+[.)] |> ?)?/;

// applySlash writes item in place of the menu's query from query to to,
// and the slash before it.
export function applySlash(
  state: EditorState,
  item: SlashItem,
  query: number,
  to: number,
): TransactionSpec {
  const from = state.sliceDoc(query - 1, query) === "/" ? query - 1 : query;
  const line = state.doc.lineAt(from);
  const rest =
    line.text.slice(0, from - line.from) + line.text.slice(to - line.from);
  const at = item.text.indexOf("‸");
  const text = item.text.replace("‸", "");
  if (item.kind === "inline")
    return {
      changes: { from, to, insert: text },
      selection: EditorSelection.cursor(from + at),
      userEvent: "input.complete",
    };
  if (item.kind === "line") {
    const mark = lineMark.exec(rest)?.[0] ?? "";
    const indent = /^\s*/.exec(rest)?.[0] ?? "";
    const next = indent + text + rest.slice(mark.length);
    const cursor = from - line.from - mark.length + indent.length + text.length;
    return {
      changes: { from: line.from, to: line.to, insert: next },
      selection: EditorSelection.cursor(
        line.from + Math.max(cursor, indent.length + text.length),
      ),
      userEvent: "input.complete",
    };
  }
  // A block takes its own line: the current one when the slash was all it
  // held, else a new line after it.
  const own = rest.trim() === "";
  const before = own ? "" : `${rest}\n`;
  return {
    changes: { from: line.from, to: line.to, insert: before + text },
    selection: EditorSelection.cursor(line.from + before.length + at),
    userEvent: "input.complete",
  };
}

// slashCompletions is the menu's completion source: after a slash that
// starts a word outside code and links, or wherever it is asked for.
export function slashCompletions(
  context: CompletionContext,
): CompletionResult | null {
  const { state, pos } = context;
  const typed = context.matchBefore(/\/[\w ]*$/);
  let from = pos;
  if (typed) {
    const before = state.sliceDoc(typed.from - 1, typed.from);
    if (typed.from > state.doc.lineAt(pos).from && !/\s/.test(before))
      return null;
    from = typed.from + 1;
  } else if (!context.explicit) return null;
  if (inLinkOrCode(state, from)) return null;
  // The query after the slash filters the menu by label; choosing an item
  // replaces the slash too.
  return {
    from,
    options: slashItems.map(
      (item, index): Completion => ({
        label: item.label,
        detail: item.detail,
        // The menu keeps this order rather than sorting by label.
        sortText: String(index).padStart(2, "0"),
        apply: (view, _completion, start, end) =>
          view.dispatch(applySlash(view.state, item, start, end)),
      }),
    ),
    validFor: /^[\w ]*$/,
  };
}
