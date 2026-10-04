// Markdown helpers for ticket and review files: link resolution (CARD-2)
// and trimming sections the card panel shows elsewhere.

export type LinkTarget =
  | { kind: "ticket"; project: string; slug: string }
  | { kind: "external"; href: string }
  | { kind: "attachment"; href: string }
  | { kind: "none" };

export interface LinkContext {
  project: string;
  base: "tickets" | "reviews";
}

const safeSegment = /^[A-Za-z0-9][A-Za-z0-9._ -]*$/;

export function resolveLink(href: string, context: LinkContext): LinkTarget {
  const trimmed = href.trim();
  if (/^https?:\/\//i.test(trimmed)) return { kind: "external", href: trimmed };
  if (
    /^[a-z][a-z0-9+.-]*:/i.test(trimmed) ||
    trimmed.startsWith("#") ||
    trimmed.startsWith("/")
  ) {
    return { kind: "none" };
  }
  const path = decodeSafely(trimmed.split(/[?#]/)[0] ?? "");
  const parts = path.split("/").filter((part) => part !== "" && part !== ".");

  const attachment = parts.indexOf("attachments");
  if (attachment >= 0) {
    const [ticket, file, ...rest] = parts.slice(attachment + 1);
    if (
      ticket &&
      file &&
      rest.length === 0 &&
      safeSegment.test(ticket) &&
      safeSegment.test(file)
    ) {
      return {
        kind: "attachment",
        href: `/api/projects/${encodeURIComponent(context.project)}/attachments/${encodeURIComponent(ticket)}/${encodeURIComponent(file)}`,
      };
    }
    return { kind: "none" };
  }

  const last = parts[parts.length - 1];
  if (last?.endsWith(".md")) {
    const slug = last.slice(0, -3);
    if (
      safeSegment.test(slug) &&
      !parts.slice(0, -1).some((part) => part !== ".." && !isColumnDir(part))
    ) {
      return { kind: "ticket", project: context.project, slug };
    }
  }
  return { kind: "none" };
}

function isColumnDir(part: string): boolean {
  return ["todo", "in-progress", "ready-to-review", "done", "reviews"].includes(
    part,
  );
}

function decodeSafely(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
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
