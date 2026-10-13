import { ensureSyntaxTree } from "@codemirror/language";
import { EditorSelection, EditorState } from "@codemirror/state";
import type { DecorationSet } from "@codemirror/view";
import { describe, expect, it } from "vitest";

import { markdownSupport } from "./language";
import { activeLines, previewDecorations } from "./preview";

const context = { project: "flashheart", ticket: "FH-16" };
const keys = new Set(["FH"]);

function stateOf(doc: string, cursor?: number) {
  const state = EditorState.create({
    doc,
    selection:
      cursor === undefined ? undefined : EditorSelection.cursor(cursor),
    extensions: [markdownSupport],
  });
  ensureSyntaxTree(state, state.doc.length, 5000);
  return state;
}

// seen lists a decoration set as words a test can read: hidden syntax,
// marks and line classes with the text they cover, and widgets by name.
function seen(state: EditorState, set: DecorationSet): string[] {
  const out: string[] = [];
  set.between(0, state.doc.length, (from, to, decoration) => {
    const spec = decoration.spec as {
      kind?: string;
      class?: string;
      widget?: { name: string };
    };
    const text = JSON.stringify(state.sliceDoc(from, to));
    if (spec.kind === "line")
      out.push(`line ${spec.class} ${state.doc.lineAt(from).number}`);
    else if (spec.kind === "hide") out.push(`hide ${text}`);
    else if (spec.widget) out.push(`${spec.widget.name} ${text}`);
    else out.push(`${spec.class} ${text}`);
  });
  return out;
}

// preview decorates doc with no line active (the editor unfocused), or
// with the line holding cursor active.
function preview(doc: string, cursor?: number) {
  const state = stateOf(doc, cursor);
  const active = activeLines(state, cursor !== undefined);
  return seen(state, previewDecorations(state, active, { context, keys }));
}

describe("the live preview", () => {
  it("hides a heading's marks except on the line being edited", () => {
    expect(preview("### Plan\n\ntext")).toEqual([
      "line fh-h3 1",
      'hide "### "',
    ]);
    const editing = preview("### Plan\n\ntext", 2);
    expect(editing).toContain("line fh-h3 1");
    expect(editing).toContain('fh-syntax "###"');
    expect(editing).not.toContain('hide "### "');
  });

  it("formats emphasis, strong, strikethrough and code", () => {
    const shown = preview("Some **bold**, *em*, ~~gone~~ and `code`.");
    expect(shown).toEqual(
      expect.arrayContaining([
        'fh-strong "**bold**"',
        'hide "**"',
        'fh-em "*em*"',
        'hide "*"',
        'fh-strike "~~gone~~"',
        'hide "~~"',
        'fh-code "`code`"',
        'hide "`"',
      ]),
    );
    // A single tilde strikes through as remark-gfm reads it in the view.
    expect(preview("a ~gone~ b")).toContain('fh-strike "~gone~"');
  });

  it("draws task markers as checkboxes and bullets as bullets", () => {
    const shown = preview("- [ ] open\n- [x] done\n- plain\n1. first");
    expect(shown).toEqual(
      expect.arrayContaining([
        'hide "- "',
        'checkbox(open) "[ ]"',
        'checkbox(done) "[x]"',
        'bullet "-"',
        'fh-list-mark "1."',
      ]),
    );
    // On the line being edited the task shows as written.
    const editing = preview("- [ ] open\n- [x] done", 3);
    expect(editing).not.toContain('checkbox(open) "[ ]"');
    expect(editing).toContain('checkbox(done) "[x]"');
  });

  it("hides a quote's marks and styles its lines", () => {
    expect(preview("> quoted\n> more")).toEqual([
      "line fh-quote 1",
      'hide "> "',
      "line fh-quote 2",
      'hide "> "',
    ]);
  });

  it("shows fenced code and tables as text on styled lines", () => {
    const code = preview("```ts\nconst a = 1;\n```");
    expect(code).toEqual([
      "line fh-fence 1",
      "line fh-codeblock 2",
      "line fh-fence 3",
    ]);
    const table = preview("| A | B |\n| --- | --- |\n| 1 | 2 |");
    expect(table).toEqual([
      "line fh-table 1",
      "line fh-table 2",
      "line fh-table 3",
    ]);
  });

  it("shows a link's words and hides its target", () => {
    expect(preview("See [the spec](https://example.com/spec).")).toEqual([
      'hide "["',
      'fh-link "the spec"',
      'hide "](https://example.com/spec)"',
    ]);
    // A ticket's own file and another ticket link too (CARD-2).
    expect(preview("[log](files/run.log)")).toContain('fh-link "log"');
    expect(preview("[other](../FH-4-mcp/FH-4-mcp.md)")).toContain(
      'fh-ticket "other"',
    );
    // A link with a scheme the board does not open is words only.
    const unsafe = preview("[x](javascript:alert(1))");
    expect(unsafe).toContain('hide "["');
    expect(unsafe.some((item) => item.startsWith("fh-link"))).toBe(false);
  });

  it("never hides a line break: a link written across lines keeps the part that would", () => {
    const shown = preview(
      "[two\nlines](https://example.com/x) and ![a\nb](files/x.png)",
    );
    expect(
      shown.filter(
        (item) => /^(hide|image)/.test(item) && item.includes("\\n"),
      ),
    ).toEqual([]);
    expect(shown).toContain('hide "["');
  });

  it("links ticket ids of known projects outside code and links", () => {
    expect(preview("Depends on FH-42 and XY-1.")).toEqual([
      'fh-ticket "FH-42"',
    ]);
    expect(preview("`FH-42`").some((item) => item.includes("ticket"))).toBe(
      false,
    );
    expect(preview("https://example.com/FH-42")).toEqual([
      'fh-link "https://example.com/FH-42"',
    ]);
  });

  it("shows the ticket's own images and never a remote one", () => {
    expect(preview("![shot](files/shot.png)")).toEqual([
      'image(/api/projects/flashheart/tickets/FH-16/files/shot.png) "![shot](files/shot.png)"',
    ]);
    expect(preview("![x](https://evil.example/x.png)")).toEqual([
      'image-unavailable(x) "![x](https://evil.example/x.png)"',
    ]);
    // On the line being edited the image is its markdown.
    expect(preview("![shot](files/shot.png)", 1)).not.toContain(
      'image(/api/projects/flashheart/tickets/FH-16/files/shot.png) "![shot](files/shot.png)"',
    );
  });

  it("draws a thematic break as a rule and hides escapes", () => {
    expect(preview("a\n\n---\n\nb")).toEqual(['rule "---"']);
    expect(preview("get\\_ticket")).toEqual(['hide "\\\\"']);
  });

  it("marks headings for screen readers", () => {
    const state = stateOf("### Plan");
    const set = previewDecorations(state, new Set(), { context, keys });
    let attributes: Record<string, string> | undefined;
    set.between(0, 1, (_from, _to, decoration) => {
      attributes ??= decoration.spec.attributes;
    });
    expect(attributes).toEqual({ role: "heading", "aria-level": "3" });
  });
});

describe("activeLines", () => {
  it("is every line a selection touches, while the editor has focus", () => {
    const state = EditorState.create({
      doc: "one\ntwo\nthree\nfour",
      selection: EditorSelection.range(5, 10),
    });
    expect([...activeLines(state, true)]).toEqual([2, 3]);
    expect(activeLines(state, false).size).toBe(0);
  });
});
