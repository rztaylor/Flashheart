import { insertNewlineContinueMarkup } from "@codemirror/lang-markdown";
import { ensureSyntaxTree } from "@codemirror/language";
import {
  EditorSelection,
  EditorState,
  type StateCommand,
} from "@codemirror/state";
import { describe, expect, it } from "vitest";

import { markdownSections } from "../../model/markdown";
import { sameSection, sectionSource } from "../../model/sectionEdit";
import { toggleBold, toggleTaskOnLine } from "./commands";
import { previewDecorations } from "./preview";
import { editorExtensions, modeCompartment, modeExtensions } from "./setup";

// The editor against real tickets (FH-16, criterion 4): every section a
// person can edit in place, from the read-only copies of FH tickets in
// testdata/tickets (FH-66's corpus, which the Go tests also replay through
// the server's section save) and a sample of every GFM construct. Opening
// a section and saving it untouched must give back its text exactly (so
// the page sends nothing), and a small edit made through the editor must
// change only what it meant to. The server writes only that section
// (board-format §Section edits), so the rest of the file stays byte for
// byte.

const files: Record<string, string> = import.meta.glob(
  "../../../../testdata/tickets/FH-*.md",
  { query: "?raw", import: "default", eager: true },
);
const tickets = Object.entries(files).map(([path, text]) => ({
  name: path.slice(path.lastIndexOf("/") + 1),
  text,
}));

const gfm = [
  "Intro with **bold**, *em*, _em_, ~~gone~~, `get_ticket` and a\\_escape.",
  "",
  "- [x] A done criterion",
  "- [ ] An open one naming FH-42",
  "- A plain bullet",
  "  - nested",
  "",
  "1. first",
  "2. second",
  "",
  "> a quote",
  "",
  "```go",
  "func main() {}",
  "```",
  "",
  "| Column | Other |",
  "| ------ | ----- |",
  "| a      | b     |",
  "",
  "![screenshot](files/shot.png) and [a log](files/run.log), [FH-4](../FH-4-mcp/FH-4-mcp.md),",
  "https://example.com/x <id> &lt;id&gt; trailing two spaces  ",
  "hard break above",
  "",
  "---",
].join("\n");

// The sections a person edits in place: every level-2 section but Handoff
// and Notes (board.NewSectionChange).
const sections = [
  ...tickets.flatMap(({ name, text }) =>
    markdownSections(text.replace(/\r\n/g, "\n"))
      .sections.map((section) => section.title)
      .filter((title) => !/^(handoff|notes)$/i.test(title.trim()))
      .map((title) => ({
        name: `${name} § ${title}`,
        text: sectionSource(text, title),
      })),
  ),
  { name: "GFM sample", text: gfm },
];

const options = {
  label: "Section",
  context: { project: "flashheart", ticket: "FH-16" },
  keys: new Set(["FH"]),
};

function open(text: string) {
  const state = EditorState.create({
    doc: text,
    extensions: [
      editorExtensions(options, "formatted"),
      EditorState.allowMultipleSelections.of(true),
    ],
  });
  ensureSyntaxTree(state, state.doc.length, 5000);
  return state;
}

function run(state: EditorState, command: StateCommand) {
  let next = state;
  command({
    state,
    dispatch: (transaction) => {
      next = transaction.state;
    },
  });
  return next;
}

// changedLines lists the lines of after that differ from before's, and
// how many lines were added.
function lineDiff(before: string, after: string) {
  const a = before.split("\n");
  const b = after.split("\n");
  let start = 0;
  while (start < a.length && a[start] === b[start]) start++;
  let endA = a.length;
  let endB = b.length;
  while (endA > start && endB > start && a[endA - 1] === b[endB - 1]) {
    endA--;
    endB--;
  }
  return { removed: a.slice(start, endA), added: b.slice(start, endB) };
}

describe("the editor over real ticket sections", () => {
  it("has a corpus to check", () => {
    expect(tickets.length).toBeGreaterThanOrEqual(6);
    expect(sections.length).toBeGreaterThanOrEqual(25);
  });

  it.each(sections)("opens and saves $name untouched, exactly", ({ text }) => {
    let state = open(text);
    expect(state.doc.toString()).toBe(text);
    // Drawing it formatted, and switching to markdown and back, never
    // touches the text.
    previewDecorations(state, new Set(), options);
    for (const mode of ["markdown", "formatted"] as const)
      state = state.update({
        effects: modeCompartment.reconfigure(modeExtensions(mode, options)),
      }).state;
    expect(state.doc.toString()).toBe(text);
    expect(sameSection(state.doc.toString(), text)).toBe(true);
  });

  it.each(sections)(
    "changes only what a small edit in $name meant to",
    ({ text }) => {
      const state = open(text);
      // Replace one word of five letters or more that occurs once.
      const word = [...new Set(text.match(/\b[A-Za-z]{5,}\b/g) ?? [])].find(
        (candidate) =>
          text.match(new RegExp(`(?<![\\w-])${candidate}(?![\\w-])`, "g"))
            ?.length === 1,
      );
      if (word) {
        const from = text.search(new RegExp(`(?<![\\w-])${word}(?![\\w-])`));
        const edited = state
          .update({
            changes: { from, to: from + word.length, insert: "Replaced" },
          })
          .state.doc.toString();
        const { removed, added } = lineDiff(text, edited);
        expect(removed).toHaveLength(1);
        expect(added).toEqual([removed[0]?.replace(word, "Replaced")]);

        // Bold it from the toolbar's command.
        const bold = run(
          state.update({
            selection: EditorSelection.range(from, from + word.length),
          }).state,
          toggleBold,
        ).doc.toString();
        const boldDiff = lineDiff(text, bold);
        expect(boldDiff.added).toEqual([
          boldDiff.removed[0]?.replace(word, `**${word}**`),
        ]);
      }

      // Tick the first task, as its checkbox does: one character.
      const taskLine = text
        .split("\n")
        .findIndex((line) => /^\s*[-*+] \[[ xX]\] /.test(line));
      if (taskLine >= 0) {
        const tick = toggleTaskOnLine(state, taskLine + 1);
        if (tick) {
          const ticked = state.update(tick).state.doc.toString();
          expect(ticked.length).toBe(text.length);
          const { removed, added } = lineDiff(text, ticked);
          expect(removed).toHaveLength(1);
          expect(added).toHaveLength(1);
        }
      }

      // Add an item after the first list item: Enter continues the list.
      const itemLine = text
        .split("\n")
        .findIndex((line) => /^\s*[-*+] /.test(line));
      if (itemLine >= 0) {
        const end = state.doc.line(itemLine + 1).to;
        let next = state.update({
          selection: EditorSelection.cursor(end),
        }).state;
        next = run(next, insertNewlineContinueMarkup);
        next = next.update(next.replaceSelection("New item")).state;
        const { removed, added } = lineDiff(text, next.doc.toString());
        expect(removed).toEqual([]);
        expect(added).toHaveLength(1);
        expect(added[0]).toMatch(/^\s*[-*+] (\[ \] )?New item$/);
      }
    },
  );
});
