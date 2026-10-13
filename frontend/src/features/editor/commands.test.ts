import { ensureSyntaxTree } from "@codemirror/language";
import {
  EditorSelection,
  EditorState,
  type StateCommand,
} from "@codemirror/state";
import { describe, expect, it } from "vitest";

import {
  insertLink,
  toggleBold,
  toggleCode,
  toggleItalic,
  toggleLines,
  toggleTaskAt,
  toggleTaskOnLine,
} from "./commands";
import { markdownSupport } from "./language";

// A document in these tests marks its selection with « and », or its
// cursor with |.
function stateOf(marked: string) {
  const from = marked.search(/[«|]/);
  const doc = marked.replace(/[«»|]/g, "");
  const end = marked.indexOf("»");
  const selection =
    end < 0
      ? EditorSelection.cursor(from)
      : EditorSelection.range(from, end - 1);
  const state = EditorState.create({
    doc,
    selection,
    extensions: [markdownSupport, EditorState.allowMultipleSelections.of(true)],
  });
  ensureSyntaxTree(state, state.doc.length, 5000);
  return state;
}

function marked(state: EditorState) {
  const { from, to } = state.selection.main;
  const doc = state.doc.toString();
  if (from === to) return `${doc.slice(0, from)}|${doc.slice(from)}`;
  return `${doc.slice(0, from)}«${doc.slice(from, to)}»${doc.slice(to)}`;
}

function run(command: StateCommand, text: string) {
  let state = stateOf(text);
  const done = command({
    state,
    dispatch: (transaction) => {
      state = transaction.state;
    },
  });
  expect(done).toBe(true);
  return marked(state);
}

describe("inline formatting", () => {
  it("wraps the selection, and unwraps it again", () => {
    expect(run(toggleBold, "a «word» b")).toBe("a **«word»** b");
    expect(run(toggleBold, "a **«word»** b")).toBe("a «word» b");
    expect(run(toggleItalic, "a «word» b")).toBe("a *«word»* b");
    expect(run(toggleItalic, "a *«word»* b")).toBe("a «word» b");
    expect(run(toggleCode, "call «get_ticket»")).toBe("call `«get_ticket»`");
  });

  it("tells italic from bold around the selection", () => {
    expect(run(toggleItalic, "**«word»**")).toBe("***«word»***");
    expect(run(toggleBold, "***«word»***")).toBe("*«word»*");
    expect(run(toggleItalic, "***«word»***")).toBe("**«word»**");
  });

  it("puts the cursor between new marks when nothing is selected", () => {
    expect(run(toggleBold, "a | b")).toBe("a **|** b");
  });

  it("makes a link of the selected words, or of a selected address", () => {
    expect(run(insertLink, "see «the spec»")).toBe("see [the spec](|)");
    expect(run(insertLink, "«https://example.com»")).toBe(
      "[|](https://example.com)",
    );
    expect(run(insertLink, "a |")).toBe("a [|]()");
  });
});

describe("line formatting", () => {
  it("makes lines a list, a task list or a quote, and back", () => {
    expect(run(toggleLines("bullet"), "«one\ntwo»")).toBe("«- one\n- two»");
    expect(run(toggleLines("bullet"), "«- one\n- two»")).toBe("«one\ntwo»");
    expect(run(toggleLines("task"), "«- one\ntwo»")).toBe(
      "«- [ ] one\n- [ ] two»",
    );
    expect(run(toggleLines("task"), "- [x] o|ne")).toBe("o|ne");
    expect(run(toggleLines("ordered"), "«one\ntwo»")).toBe("«1. one\n2. two»");
    expect(run(toggleLines("quote"), "o|ne")).toBe("> o|ne");
  });

  it("keeps a line's indentation", () => {
    expect(run(toggleLines("task"), "  - n|ested")).toBe("  - [ ] n|ested");
  });
});

describe("ticking a task", () => {
  it("ticks and unticks the task marker it is given, changing one character", () => {
    const state = stateOf("- [ ] open\n- [X] done");
    const tick = toggleTaskAt(state, 2);
    expect(tick).not.toBeNull();
    expect(state.update(tick ?? {}).state.doc.toString()).toBe(
      "- [x] open\n- [X] done",
    );
    const untick = toggleTaskAt(state, 13);
    expect(state.update(untick ?? {}).state.doc.toString()).toBe(
      "- [ ] open\n- [ ] done",
    );
  });

  it("ticks the task on a line, and nothing that only looks like one", () => {
    const state = stateOf("- [ ] open\n\n```\n- [ ] code\n```\n\nplain");
    expect(
      state.update(toggleTaskOnLine(state, 1) ?? {}).state.doc.toString(),
    ).toContain("- [x] open");
    expect(toggleTaskOnLine(state, 4)).toBeNull();
    expect(toggleTaskOnLine(state, 7)).toBeNull();
  });
});
