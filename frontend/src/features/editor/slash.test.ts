import { CompletionContext } from "@codemirror/autocomplete";
import { ensureSyntaxTree } from "@codemirror/language";
import { EditorSelection, EditorState } from "@codemirror/state";
import { describe, expect, it } from "vitest";

import { markdownSupport } from "./language";
import { applySlash, slashCompletions, slashItems } from "./slash";

function stateAt(marked: string) {
  const at = marked.indexOf("|");
  const state = EditorState.create({
    doc: marked.replace("|", ""),
    selection: EditorSelection.cursor(at),
    extensions: [markdownSupport],
  });
  ensureSyntaxTree(state, state.doc.length, 5000);
  return state;
}

function menu(marked: string, explicit = false) {
  const state = stateAt(marked);
  return slashCompletions(
    new CompletionContext(state, state.selection.main.head, explicit),
  );
}

// choose applies the item labelled label to the slash text before the
// cursor, and shows the result with its cursor as ‸.
function choose(marked: string, label: string) {
  const state = stateAt(marked);
  const found = menu(marked);
  const item = slashItems.find((option) => option.label === label);
  if (!found || !item) throw new Error(`no ${label} in the menu`);
  const next = state.update(
    applySlash(state, item, found.from, state.selection.main.head),
  ).state;
  const head = next.selection.main.head;
  const doc = next.doc.toString();
  return `${doc.slice(0, head)}‸${doc.slice(head)}`;
}

describe("the slash menu", () => {
  it("opens on a slash at the start of a line or after a space", () => {
    // The menu's query, which filters it, starts after the slash.
    expect(menu("/|")?.from).toBe(1);
    expect(menu("Some text /ta|")?.from).toBe(11);
    expect(menu("files/|")).toBeNull();
    expect(menu("```\n/|\n```")).toBeNull();
    expect(menu("no slash|")).toBeNull();
    // Asked for (Ctrl+Space), it opens at the cursor without a slash.
    expect(menu("text |", true)?.from).toBe(5);
  });

  it("offers the blocks a ticket uses, with no level-2 heading", () => {
    expect(menu("/|")?.options.map((option) => option.label)).toEqual([
      "Heading",
      "Subheading",
      "Bulleted list",
      "Numbered list",
      "Task list",
      "Quote",
      "Code block",
      "Table",
      "Divider",
      "Link",
    ]);
  });

  it("turns the line into the chosen block, removing the slash text", () => {
    expect(choose("/head|", "Heading")).toBe("### ‸");
    expect(choose("Ship it /task|", "Task list")).toBe("- [ ] Ship it ‸");
    expect(choose("- item /num|", "Numbered list")).toBe("1. item ‸");
    expect(choose("/|", "Code block")).toBe("```\n‸\n```");
    expect(choose("Words /div|", "Divider")).toBe("Words \n---\n‸");
    expect(choose("See /li|", "Link")).toBe("See [‸]()");
    expect(choose("/tab|", "Table")).toBe(
      "| Column | Column |\n| --- | --- |\n| ‸ |  |",
    );
  });
});
