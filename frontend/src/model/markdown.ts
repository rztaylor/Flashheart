// Markdown helpers for ticket and review files: link resolution (CARD-2),
// linking bare ticket ids in markdown and plain text (KEY-3), the ticket's
// full page, trimming sections the card panel shows elsewhere, and for the
// full page: splitting a file into its sections and finding the images and
// the ticket's own files it links.

import { TICKET_ID } from "../api/board";

export type LinkTarget =
  | { kind: "ticket"; id: string }
  | { kind: "external"; href: string }
  | { kind: "attachment"; href: string }
  | { kind: "none" };

// LinkContext is where the markdown lives: a ticket's folder, which holds
// <id>-<slug>.md, review.md and files/.
export interface LinkContext {
  project: string;
  ticket: string;
}

const ticketID = TICKET_ID;
const ticketFolder = /^([A-Z][A-Z0-9]{1,9}-[1-9][0-9]*)(?:-.*)?$/;
const safeSegment = /^[A-Za-z0-9][A-Za-z0-9._ -]*$/;

// ticketLinkPrefix marks links made by linkTicketIds.
export const ticketLinkPrefix = "#ticket-";

// ticketPageHref is a ticket's full page (CARD-7), the target of every
// ticket id link.
export function ticketPageHref(id: string): string {
  return `#/ticket/${encodeURIComponent(id)}`;
}

// workstreamPageHref is a workstream's page (FH-139), opened from its
// route card.
export function workstreamPageHref({
  project,
  slug,
}: {
  project: string;
  slug: string;
}): string {
  return `#/workstream/${encodeURIComponent(project)}/${encodeURIComponent(slug)}`;
}

export function resolveLink(href: string, context: LinkContext): LinkTarget {
  const trimmed = href.trim();
  if (trimmed.startsWith(ticketLinkPrefix)) {
    const id = trimmed.slice(ticketLinkPrefix.length);
    return ticketID.test(id) ? { kind: "ticket", id } : { kind: "none" };
  }
  if (/^https?:\/\//i.test(trimmed)) return { kind: "external", href: trimmed };
  if (
    /^[a-z][a-z0-9+.-]*:/i.test(trimmed) ||
    trimmed.startsWith("#") ||
    trimmed.startsWith("/")
  ) {
    return { kind: "none" };
  }
  const parts = decodeSafely(trimmed.split(/[?#]/)[0] ?? "")
    .split("/")
    .filter((part) => part !== "" && part !== ".");

  // Resolve the folder the path points into: this ticket, or ../<other>/.
  let id = context.ticket;
  let rest = parts;
  if (parts[0] === "..") {
    const match = ticketFolder.exec(parts[1] ?? "");
    if (!match?.[1]) return { kind: "none" };
    id = match[1];
    rest = parts.slice(2);
  }
  // The ticket file is named after its folder, <id>-<slug>.md.
  const file = rest.length === 1 ? ticketFolder.exec(rest[0] ?? "") : null;
  if (file?.[1] === id && rest[0]?.endsWith(".md"))
    return { kind: "ticket", id };
  // A workstream's text (ticket "") has no files of its own.
  if (
    id &&
    rest.length === 2 &&
    rest[0] === "files" &&
    rest[1] &&
    safeSegment.test(rest[1])
  ) {
    return {
      kind: "attachment",
      href: `/api/projects/${encodeURIComponent(context.project)}/tickets/${encodeURIComponent(id)}/files/${encodeURIComponent(rest[1])}`,
    };
  }
  return { kind: "none" };
}

function decodeSafely(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

interface MdNode {
  type: string;
  value?: string;
  url?: string;
  children?: MdNode[];
}

const idInText = /\b([A-Z][A-Z0-9]{1,9})-([1-9][0-9]*)\b(?![A-Za-z0-9-])/g;

// ticketRefs splits plain text (a criterion, a handoff step, a blocking
// reason) into text and the ticket ids in it with a known project key.
export function ticketRefs(
  text: string,
  keys: Set<string>,
): (string | { id: string })[] {
  const parts: (string | { id: string })[] = [];
  let last = 0;
  for (const match of text.matchAll(idInText)) {
    if (!keys.has(match[1] ?? "") || match.index === undefined) continue;
    if (match.index > last) parts.push(text.slice(last, match.index));
    parts.push({ id: match[0] });
    last = match.index + match[0].length;
  }
  if (last < text.length) parts.push(text.slice(last));
  return parts;
}

// linkTicketIds is a remark plugin that turns ticket ids with a known project
// key in text ("see AL-4") into links, leaving code and existing links alone.
export function linkTicketIds(keys: Set<string>) {
  const visit = (node: MdNode) => {
    if (!node.children || node.type === "link" || node.type === "linkReference")
      return;
    const next: MdNode[] = [];
    for (const child of node.children) {
      if (child.type !== "text" || !child.value) {
        visit(child);
        next.push(child);
        continue;
      }
      let last = 0;
      for (const match of child.value.matchAll(idInText)) {
        if (!keys.has(match[1] ?? "") || match.index === undefined) continue;
        if (match.index > last)
          next.push({
            type: "text",
            value: child.value.slice(last, match.index),
          });
        next.push({
          type: "link",
          url: ticketLinkPrefix + match[0],
          children: [{ type: "text", value: match[0] }],
        });
        last = match.index + match[0].length;
      }
      if (last === 0) next.push(child);
      else if (last < child.value.length)
        next.push({ type: "text", value: child.value.slice(last) });
    }
    node.children = next;
  };
  return (tree: MdNode) => visit(tree);
}

const fence = /^ {0,3}(```|~~~)/;
const heading = /^ {0,3}(#{1,6})[ \t]+(.*?)[ \t]*#*[ \t]*$/;

// Sections the card panel renders on their own, above the body.
const separateSections = ["handoff", "acceptance criteria"];

// ticketBody removes the H1 title and the sections the card panel renders
// separately (handoff, acceptance criteria), leaving fenced code untouched.
export function ticketBody(body: string): string {
  const lines = body.split("\n");
  const kept: string[] = [];
  let inFence = false;
  let skipping = false;
  let titleDropped = false;
  for (const line of lines) {
    if (fence.test(line)) inFence = !inFence;
    const match = !inFence && !fence.test(line) ? heading.exec(line) : null;
    if (match) {
      const level = match[1]?.length ?? 0;
      if (level === 1 && !titleDropped) {
        titleDropped = true;
        continue;
      }
      if (level <= 2) {
        skipping =
          level === 2 &&
          separateSections.includes(match[2]?.toLowerCase() ?? "");
      }
    }
    if (!skipping) kept.push(line);
  }
  return kept.join("\n").trim();
}

export interface MarkdownSection {
  // title is the heading's text as written.
  title: string;
  markdown: string;
}

// markdownSections splits markdown at its level-2 headings outside code
// fences, for the ticket's full page (CARD-7): lead is the text before the
// first one, without a level-1 title that opens the file, and each section
// its heading's text and its body, trimmed.
export function markdownSections(markdown: string): {
  lead: string;
  sections: MarkdownSection[];
} {
  const lead: string[] = [];
  const sections: { title: string; lines: string[] }[] = [];
  let inFence = false;
  let opened = false;
  for (const line of markdown.split("\n")) {
    const fenceLine = fence.test(line);
    const match = !inFence && !fenceLine ? heading.exec(line) : null;
    if (fenceLine) inFence = !inFence;
    const level = match?.[1]?.length ?? 0;
    if (level === 1 && !opened) {
      opened = true;
      continue;
    }
    if (line.trim()) opened = true;
    if (level === 2) {
      sections.push({ title: match?.[2] ?? "", lines: [] });
      continue;
    }
    const current = sections[sections.length - 1];
    (current ? current.lines : lead).push(line);
  }
  return {
    lead: lead.join("\n").trim(),
    sections: sections.map((section) => ({
      title: section.title,
      markdown: section.lines.join("\n").trim(),
    })),
  };
}

export interface ImageRef {
  alt: string;
  src: string;
}

const image = /!\[([^\]]*)\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)/g;
const link = /\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)/g;
const codeSpan = /`[^`]*`/g;
const emptyItem = /^\s*(?:[-*+]|\d+[.)])\s*$/;

// outsideCode calls visit with each line outside fenced code, its code
// spans blanked, so an image written as code is not taken for one.
function outsideCode(markdown: string, visit: (line: string) => void) {
  let inFence = false;
  for (const line of markdown.split("\n")) {
    if (fence.test(line)) {
      inFence = !inFence;
      continue;
    }
    if (!inFence)
      visit(line.replace(codeSpan, (span) => " ".repeat(span.length)));
  }
}

// imageRefs lists the images markdown shows, in order, outside code.
export function imageRefs(markdown: string): ImageRef[] {
  const refs: ImageRef[] = [];
  outsideCode(markdown, (line) => {
    for (const match of line.matchAll(image))
      refs.push({ alt: match[1] ?? "", src: match[2] ?? "" });
  });
  return refs;
}

// withoutImages removes the images from markdown outside code, and the
// list items that held only an image, keeping the words around a gallery.
export function withoutImages(markdown: string): string {
  const kept: string[] = [];
  let inFence = false;
  for (const line of markdown.split("\n")) {
    const fenceLine = fence.test(line);
    if (fenceLine) inFence = !inFence;
    if (inFence || fenceLine) {
      kept.push(line);
      continue;
    }
    // Split around code spans: the odd parts are code, left as written.
    const parts = line.split(/(`[^`]*`)/);
    const text = parts
      .map((part, index) => (index % 2 ? part : part.replace(image, "")))
      .join("");
    if (text === line) {
      kept.push(line);
      continue;
    }
    const stripped = text
      .replace(/^(\s*(?:[-*+]|\d+[.)]))\s+/, "$1 ")
      .trimEnd();
    if (!stripped.trim() || emptyItem.test(stripped)) continue;
    kept.push(stripped);
  }
  return kept.join("\n");
}

// localFile names the file a link points to in this ticket's own files
// folder ("files/a.png" is "a.png"), or "" for anything else.
export function localFile(target: string): string {
  const path = target.trim().split(/[?#]/)[0] ?? "";
  if (/^[a-z][a-z0-9+.-]*:/i.test(path) || path.startsWith("/")) return "";
  const parts = path.split("/").filter((part) => part !== "" && part !== ".");
  if (parts.length !== 2 || parts[0] !== "files" || !parts[1]) return "";
  return decodeSafely(parts[1]);
}

// linkedFiles names the ticket's own files that markdown links to or
// shows, outside code.
export function linkedFiles(markdown: string): Set<string> {
  const names = new Set<string>();
  outsideCode(markdown, (line) => {
    for (const match of line.matchAll(link)) {
      const name = localFile(match[1] ?? "");
      if (name) names.add(name);
    }
  });
  return names;
}
