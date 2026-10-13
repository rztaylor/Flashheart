import { syntaxTree } from "@codemirror/language";
import {
  type ChangeSpec,
  EditorSelection,
  type EditorState,
  type StateCommand,
  type TransactionSpec,
} from "@codemirror/state";
import type { Command } from "@codemirror/view";

import { type LinkOptions, openTarget, targetAt } from "./links";

// The editor's formatting commands, shared by the selection toolbar, the
// slash menu and the keyboard: each edits the markdown text and nothing
// else, so the text saved is exactly the text shown in Markdown view.

// runOf counts how many times mark repeats ending at pos (before) or
// starting at pos (after).
function runOf(state: EditorState, pos: number, mark: string, before: boolean) {
  let count = 0;
  for (;;) {
    const from = before
      ? pos - (count + 1) * mark.length
      : pos + count * mark.length;
    if (from < 0 || state.sliceDoc(from, from + mark.length) !== mark)
      return count;
    count++;
  }
}

// toggleInline wraps each selection in an inline mark (** for strong, *
// for emphasis, ` for code), or removes the mark around it. With nothing
// selected it inserts the pair with the cursor between. Strong and
// emphasis share *: one around the selection is emphasis, two strong,
// three both.
function toggleInline(mark: string, unit: 1 | 2 = 1): StateCommand {
  return ({ state, dispatch }) => {
    const change = state.changeByRange((range) => {
      const char = mark[0] ?? "";
      const around = Math.min(
        runOf(state, range.from, char, true),
        runOf(state, range.to, char, false),
      );
      const wrapped =
        char === "*"
          ? unit === 2
            ? around >= 2
            : around % 2 === 1
          : around >= 1;
      if (!range.empty && wrapped) {
        const size = mark.length;
        return {
          changes: [
            { from: range.from - size, to: range.from },
            { from: range.to, to: range.to + size },
          ],
          range: EditorSelection.range(range.from - size, range.to - size),
        };
      }
      return {
        changes: [
          { from: range.from, insert: mark },
          { from: range.to, insert: mark },
        ],
        range: EditorSelection.range(
          range.from + mark.length,
          range.to + mark.length,
        ),
      };
    });
    dispatch(
      state.update(change, { scrollIntoView: true, userEvent: "input.format" }),
    );
    return true;
  };
}

export const toggleBold = toggleInline("**", 2);
export const toggleItalic = toggleInline("*", 1);
export const toggleCode = toggleInline("`");

// insertLink makes the selected words a link with the cursor where its
// address goes, or a selected address a link with the cursor where its
// words go.
export const insertLink: StateCommand = ({ state, dispatch }) => {
  const change = state.changeByRange((range) => {
    const text = state.sliceDoc(range.from, range.to);
    const address = /^https?:\/\/\S+$/.test(text);
    const insert = address ? `[](${text})` : `[${text}]()`;
    // The cursor goes where the link still needs something: its words for
    // an address or an empty link, else its address.
    const cursor =
      address || range.empty ? range.from + 1 : range.from + insert.length - 1;
    return {
      changes: { from: range.from, to: range.to, insert },
      range: EditorSelection.cursor(cursor),
    };
  });
  dispatch(
    state.update(change, { scrollIntoView: true, userEvent: "input.format" }),
  );
  return true;
};

export type LineKind = "bullet" | "task" | "ordered" | "quote";

const prefixes: Record<LineKind, RegExp> = {
  task: /^(\s*)[-*+] \[[ xX]\] /,
  bullet: /^(\s*)[-*+] (?!\[[ xX]\] )/,
  ordered: /^(\s*)\d+[.)] /,
  quote: /^(\s*)> ?/,
};
const anyList = /^(\s*)(?:[-*+] \[[ xX]\] |[-*+] |\d+[.)] )/;

// toggleLines makes every line the selection touches a bulleted list, a
// task list, a numbered list or a quote; when they all are already, it
// makes them plain lines again. Indentation is kept.
export function toggleLines(kind: LineKind): StateCommand {
  return ({ state, dispatch }) => {
    const numbers = new Set<number>();
    for (const range of state.selection.ranges) {
      const last = state.doc.lineAt(range.to).number;
      for (let n = state.doc.lineAt(range.from).number; n <= last; n++)
        numbers.add(n);
    }
    const lines = [...numbers]
      .sort((a, b) => a - b)
      .map((n) => state.doc.line(n));
    const all = lines.every((line) => prefixes[kind].test(line.text));
    const changes: ChangeSpec[] = [];
    let count = 0;
    for (const line of lines) {
      const own = kind === "quote" ? prefixes.quote : anyList;
      const current = own.exec(line.text) ?? prefixes[kind].exec(line.text);
      const indent = /^\s*/.exec(line.text)?.[0] ?? "";
      const end = line.from + (current ? current[0].length : indent.length);
      if (all) {
        changes.push({ from: line.from + indent.length, to: end });
        continue;
      }
      count++;
      const prefix =
        kind === "bullet"
          ? "- "
          : kind === "task"
            ? "- [ ] "
            : kind === "ordered"
              ? `${count}. `
              : "> ";
      changes.push({
        from: line.from + indent.length,
        to: end,
        insert: prefix,
      });
    }
    const changeSet = state.changes(changes);
    const selection = EditorSelection.create(
      state.selection.ranges.map((range) =>
        range.empty
          ? EditorSelection.cursor(changeSet.mapPos(range.from, 1))
          : EditorSelection.range(
              changeSet.mapPos(range.from, -1),
              changeSet.mapPos(range.to, 1),
            ),
      ),
      state.selection.mainIndex,
    );
    dispatch(
      state.update({
        changes: changeSet,
        selection,
        scrollIntoView: true,
        userEvent: "input.format",
      }),
    );
    return true;
  };
}

// toggleTaskAt ticks or unticks the task marker ([ ] or [x]) that starts
// at pos, changing that one character; null where no task marker starts.
export function toggleTaskAt(
  state: EditorState,
  pos: number,
): TransactionSpec | null {
  const node = syntaxTree(state).resolveInner(pos, 1);
  if (node.name !== "TaskMarker" || node.from !== pos) return null;
  const done = state.sliceDoc(pos + 1, pos + 2) !== " ";
  return {
    changes: { from: pos + 1, to: pos + 2, insert: done ? " " : "x" },
    userEvent: "input.toggle",
  };
}

// toggleTaskOnLine ticks or unticks the task on line number n.
export function toggleTaskOnLine(
  state: EditorState,
  n: number,
): TransactionSpec | null {
  const line = state.doc.line(n);
  const match = /^\s*(?:[-*+]|\d+[.)]) +(?=\[[ xX]\])/.exec(line.text);
  return match ? toggleTaskAt(state, line.from + match[0].length) : null;
}

// activate acts on what the cursor is on: it opens a link or ticket id in
// a new tab, or else ticks the task on its line. The keyboard's way to do
// what a modified click or a checkbox click does.
export function activate(options: LinkOptions): Command {
  return (view) => {
    const { state } = view;
    const { head } = state.selection.main;
    if (openTarget(targetAt(state, head, options))) return true;
    const tick = toggleTaskOnLine(state, state.doc.lineAt(head).number);
    if (!tick) return false;
    view.dispatch(tick);
    return true;
  };
}
