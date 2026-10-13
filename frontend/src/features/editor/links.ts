import { syntaxTree } from "@codemirror/language";
import type { EditorState } from "@codemirror/state";

import {
  type LinkContext,
  type LinkTarget,
  resolveLink,
  ticketPageHref,
  ticketRefs,
} from "../../model/markdown";

// What a link in the editor points to, by the same rules as the view
// (CARD-2, model/markdown): a ticket's full page, a web page, one of the
// ticket's own files, or nothing for any other scheme or path.

export interface LinkOptions {
  context: LinkContext;
  // keys are the project keys whose ticket ids in text link (KEY-3).
  keys: Set<string>;
}

// The nodes inside which a ticket id is text, not a link.
const notText = new Set([
  "InlineCode",
  "FencedCode",
  "CodeBlock",
  "Link",
  "Image",
  "URL",
  "Autolink",
  "HTMLTag",
  "HTMLBlock",
]);

// inLinkOrCode reports a position inside code, a link or HTML.
export function inLinkOrCode(state: EditorState, pos: number): boolean {
  return ancestors(state, pos, 1).some((node) => notText.has(node.name));
}

// ancestors are the syntax nodes at pos, innermost first, entered from
// the side given (1 after pos, -1 before it).
function ancestors(state: EditorState, pos: number, side: 1 | -1) {
  const start = syntaxTree(state).resolveInner(pos, side);
  const nodes: (typeof start)[] = [];
  for (let node: typeof start | null = start; node; node = node.parent)
    nodes.push(node);
  return nodes;
}

export interface IdRange {
  from: number;
  to: number;
  id: string;
}

// ticketIds finds the ticket ids of known projects in a line of text, as
// offsets from its start.
export function ticketIds(text: string, keys: Set<string>): IdRange[] {
  const ranges: IdRange[] = [];
  let at = 0;
  for (const part of ticketRefs(text, keys)) {
    if (typeof part === "string") at += part.length;
    else {
      ranges.push({ from: at, to: at + part.id.length, id: part.id });
      at += part.id.length;
    }
  }
  return ranges;
}

// targetAt is what a link, bare web address or ticket id at pos points
// to; kind "none" where there is none.
export function targetAt(
  state: EditorState,
  pos: number,
  { context, keys }: LinkOptions,
): LinkTarget {
  for (const side of [1, -1] as const) {
    for (const node of ancestors(state, pos, side)) {
      if (node.name === "Link" || node.name === "Image") {
        const url = node.getChild("URL");
        return url
          ? resolveLink(state.sliceDoc(url.from, url.to), context)
          : { kind: "none" };
      }
      if (node.name === "URL" || node.name === "Autolink")
        return resolveLink(
          state.sliceDoc(node.from, node.to).replace(/^<|>$/g, ""),
          context,
        );
    }
  }
  const line = state.doc.lineAt(pos);
  for (const range of ticketIds(line.text, keys)) {
    const from = line.from + range.from;
    if (
      pos >= from &&
      pos <= line.from + range.to &&
      !inLinkOrCode(state, from)
    )
      return { kind: "ticket", id: range.id };
  }
  return { kind: "none" };
}

// hrefOf is where a target opens, or undefined for none.
export function hrefOf(target: LinkTarget): string | undefined {
  switch (target.kind) {
    case "ticket":
      return ticketPageHref(target.id);
    case "external":
    case "attachment":
      return target.href;
    default:
      return undefined;
  }
}

// openTarget opens a link from the editor in a new tab, so the edit in
// this one is kept; it reports whether there was anything to open.
export function openTarget(
  target: LinkTarget,
  open: (href: string) => void = (href) =>
    window.open(href, "_blank", "noopener,noreferrer"),
): boolean {
  const href = hrefOf(target);
  if (!href) return false;
  open(href);
  return true;
}
