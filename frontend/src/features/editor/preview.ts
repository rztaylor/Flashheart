import { syntaxTree } from "@codemirror/language";
import type { EditorState, Extension, Range } from "@codemirror/state";
import {
  Decoration,
  type DecorationSet,
  type EditorView,
  ViewPlugin,
  type ViewUpdate,
} from "@codemirror/view";

import { resolveLink } from "../../model/markdown";
import { inLinkOrCode, type LinkOptions, ticketIds } from "./links";
import {
  BulletWidget,
  CheckboxWidget,
  ImageWidget,
  RuleWidget,
  UnavailableImageWidget,
} from "./widgets";

// The live preview (FH-16, D61): decorations over the markdown text that
// hide its syntax and draw it formatted, except on the lines the selection
// touches while the editor has focus, where the syntax shows (faintly) to
// be edited. The document is always the markdown itself, so nothing is
// ever re-serialised: what is saved is what was typed.

// activeLines are the line numbers the selection touches, while focused.
export function activeLines(state: EditorState, focused: boolean) {
  const lines = new Set<number>();
  if (!focused) return lines;
  for (const range of state.selection.ranges) {
    const last = state.doc.lineAt(range.to).number;
    for (let n = state.doc.lineAt(range.from).number; n <= last; n++)
      lines.add(n);
  }
  return lines;
}

const hidden = Decoration.replace({ kind: "hide" });
const syntaxMark = Decoration.mark({ class: "fh-syntax", kind: "mark" });
const markOf = (name: string) => Decoration.mark({ class: name, kind: "mark" });
const lineOf = (name: string, attributes?: Record<string, string>) =>
  Decoration.line({ class: name, attributes, kind: "line" });

const strong = markOf("fh-strong");
const em = markOf("fh-em");
const strike = markOf("fh-strike");
const code = markOf("fh-code");
const link = markOf("fh-link");
const ticket = markOf("fh-ticket");
const listMark = markOf("fh-list-mark");
const headings = [1, 2, 3, 4, 5, 6].map((level) =>
  lineOf(`fh-h${level}`, { role: "heading", "aria-level": String(level) }),
);
const quoteLine = lineOf("fh-quote");
const fenceLine = lineOf("fh-fence");
const codeLine = lineOf("fh-codeblock");
const tableLine = lineOf("fh-table");

const inlineMarks: Record<string, Decoration> = {
  StrongEmphasis: strong,
  Emphasis: em,
  Strikethrough: strike,
  // remark-gfm strikes through a single-tilde span; the parser calls it
  // subscript.
  Subscript: strike,
};
const syntaxMarks = new Set([
  "EmphasisMark",
  "StrikethroughMark",
  "SubscriptMark",
]);

// previewDecorations decorates state's text within ranges (all of it by
// default), showing the syntax on the active lines.
export function previewDecorations(
  state: EditorState,
  active: ReadonlySet<number>,
  options: LinkOptions,
  ranges: readonly { from: number; to: number }[] = [
    { from: 0, to: state.doc.length },
  ],
): DecorationSet {
  const { doc } = state;
  const found: Range<Decoration>[] = [];
  const isActive = (pos: number) => active.has(doc.lineAt(pos).number);
  const add = (decoration: Decoration, from: number, to: number) => {
    if (to > from) found.push(decoration.range(from, to));
  };
  // replace hides from..to, or draws a widget there, within one line: a
  // plugin may not replace a line break (a link or image written across
  // lines keeps its markdown).
  const replace = (decoration: Decoration, from: number, to: number) => {
    if (doc.lineAt(from).number === doc.lineAt(to).number)
      add(decoration, from, to);
  };
  // syntax hides markdown syntax, or shows it faintly on an active line;
  // spaceAfter takes the space that follows it along when hidden.
  const syntax = (from: number, to: number, spaceAfter = false) => {
    if (isActive(from)) add(syntaxMark, from, to);
    else
      replace(
        hidden,
        from,
        spaceAfter && doc.sliceString(to, to + 1) === " " ? to + 1 : to,
      );
  };
  const eachLine = (from: number, to: number, decoration: Decoration) => {
    for (let pos = from; pos <= to; ) {
      const line = doc.lineAt(pos);
      found.push(decoration.range(line.from));
      pos = line.to + 1;
    }
  };

  for (const { from, to } of ranges) {
    syntaxTree(state).iterate({
      from,
      to,
      enter(node) {
        const name = node.name;
        const heading = /^(?:ATX|Setext)Heading(\d)$/.exec(name);
        if (heading) {
          const decoration = headings[Number(heading[1]) - 1];
          if (decoration)
            found.push(decoration.range(doc.lineAt(node.from).from));
          return true;
        }
        const inline = inlineMarks[name];
        if (inline) {
          add(inline, node.from, node.to);
          return true;
        }
        switch (name) {
          case "HeaderMark": {
            // An opening mark takes its space; a closing one the space
            // before it.
            const line = doc.lineAt(node.from);
            if (
              node.from === line.from ||
              /^\s*$/.test(doc.sliceString(line.from, node.from))
            )
              syntax(node.from, node.to, true);
            else if (isActive(node.from)) add(syntaxMark, node.from, node.to);
            else replace(hidden, Math.max(line.from, node.from - 1), node.to);
            return false;
          }
          case "InlineCode": {
            add(code, node.from, node.to);
            for (const mark of node.node.getChildren("CodeMark"))
              syntax(mark.from, mark.to);
            return false;
          }
          case "FencedCode": {
            const first = doc.lineAt(node.from);
            const last = doc.lineAt(node.to);
            const marks = node.node.getChildren("CodeMark");
            const closed =
              marks.length > 1 &&
              doc.lineAt(marks[marks.length - 1]?.from ?? 0).number !==
                first.number;
            found.push(fenceLine.range(first.from));
            if (last.number > first.number) {
              const body = closed ? doc.line(last.number - 1) : last;
              if (body.number > first.number)
                eachLine(doc.line(first.number + 1).from, body.from, codeLine);
              if (closed) found.push(fenceLine.range(last.from));
            }
            return false;
          }
          case "CodeBlock":
            eachLine(node.from, node.to, codeLine);
            return false;
          case "Table":
            eachLine(node.from, node.to, tableLine);
            return false;
          case "Blockquote":
            eachLine(node.from, node.to, quoteLine);
            return true;
          case "QuoteMark":
            syntax(node.from, node.to, true);
            return false;
          case "ListMark": {
            const item = node.node.parent;
            const list = item?.parent?.name;
            if (item?.getChild("Task")) syntax(node.from, node.to, true);
            else if (list === "OrderedList") add(listMark, node.from, node.to);
            else if (isActive(node.from)) add(syntaxMark, node.from, node.to);
            else
              replace(
                Decoration.replace({ widget: new BulletWidget() }),
                node.from,
                node.to,
              );
            return false;
          }
          case "TaskMarker": {
            if (isActive(node.from)) {
              add(syntaxMark, node.from, node.to);
              return false;
            }
            const checked =
              doc.sliceString(node.from + 1, node.from + 2) !== " ";
            const task = node.node.parent;
            const label = task ? doc.sliceString(node.to, task.to).trim() : "";
            replace(
              Decoration.replace({
                widget: new CheckboxWidget(checked, label),
              }),
              node.from,
              node.to,
            );
            return false;
          }
          case "Link": {
            const marks = node.node.getChildren("LinkMark");
            const url = node.node.getChild("URL");
            const open = marks[0];
            const close = marks[1];
            if (!open || !close) return false;
            const target = url
              ? resolveLink(doc.sliceString(url.from, url.to), options.context)
              : { kind: "none" as const };
            if (target.kind === "ticket") add(ticket, open.to, close.from);
            else if (target.kind !== "none") add(link, open.to, close.from);
            syntax(open.from, open.to);
            syntax(close.from, node.to);
            return false;
          }
          case "Image": {
            if (isActive(node.from)) return false;
            const marks = node.node.getChildren("LinkMark");
            const url = node.node.getChild("URL");
            const alt =
              marks[0] && marks[1]
                ? doc.sliceString(marks[0].to, marks[1].from)
                : "";
            const target = url
              ? resolveLink(doc.sliceString(url.from, url.to), options.context)
              : { kind: "none" as const };
            const widget =
              target.kind === "attachment"
                ? new ImageWidget(target.href, alt)
                : new UnavailableImageWidget(alt);
            replace(Decoration.replace({ widget }), node.from, node.to);
            return false;
          }
          case "URL": {
            // A bare web address (GFM autolink) links, as in the view.
            const target = resolveLink(
              doc.sliceString(node.from, node.to),
              options.context,
            );
            if (target.kind !== "none") add(link, node.from, node.to);
            return false;
          }
          case "HorizontalRule":
            if (!isActive(node.from))
              replace(
                Decoration.replace({ widget: new RuleWidget() }),
                node.from,
                node.to,
              );
            return false;
          case "Escape":
            syntax(node.from, node.from + 1);
            return false;
          default:
            if (syntaxMarks.has(name)) syntax(node.from, node.to);
            return true;
        }
      },
    });
    // Ticket ids of known projects link (KEY-3), outside code and links.
    for (let pos = from; pos <= to; ) {
      const line = doc.lineAt(pos);
      for (const range of ticketIds(line.text, options.keys)) {
        const start = line.from + range.from;
        if (!inLinkOrCode(state, start))
          add(ticket, start, line.from + range.to);
      }
      pos = line.to + 1;
    }
  }
  return Decoration.set(found, true);
}

// livePreview keeps the preview's decorations over the visible text.
export function livePreview(options: LinkOptions): Extension {
  const build = (view: EditorView) =>
    previewDecorations(
      view.state,
      activeLines(view.state, view.hasFocus),
      options,
      view.visibleRanges,
    );
  return ViewPlugin.fromClass(
    class {
      decorations: DecorationSet;
      constructor(view: EditorView) {
        this.decorations = build(view);
      }
      update(update: ViewUpdate) {
        if (
          update.docChanged ||
          update.viewportChanged ||
          update.selectionSet ||
          update.focusChanged ||
          syntaxTree(update.startState) !== syntaxTree(update.state)
        )
          this.decorations = build(update.view);
      }
    },
    { decorations: (plugin) => plugin.decorations },
  );
}
