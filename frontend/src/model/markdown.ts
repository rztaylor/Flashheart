// Markdown helpers for ticket and review files: link resolution (CARD-2),
// linking bare ticket ids (KEY-3), and trimming sections the card panel
// shows elsewhere.

export type LinkTarget =
  | { kind: "ticket"; id: string }
  | { kind: "external"; href: string }
  | { kind: "attachment"; href: string }
  | { kind: "none" };

// LinkContext is where the markdown lives: a ticket's folder, which holds
// ticket.md, review.md and files/.
export interface LinkContext {
  project: string;
  ticket: string;
}

const ticketID = /^[A-Z][A-Z0-9]{1,9}-[1-9][0-9]*$/;
const ticketFolder = /^([A-Z][A-Z0-9]{1,9}-[1-9][0-9]*)(?:-.*)?$/;
const safeSegment = /^[A-Za-z0-9][A-Za-z0-9._ -]*$/;

// ticketLinkPrefix marks links made by linkTicketIds.
export const ticketLinkPrefix = "#ticket-";

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
  if (rest.length === 1 && rest[0] === "ticket.md")
    return { kind: "ticket", id };
  if (
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
