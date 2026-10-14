import type { WorkstreamBrief } from "../api/board";

// Workstream line assignment: each workstream in a project gets one line
// colour (index into the --line-N tokens) and a bullet label.

export const LINE_COUNT = 9;

export interface Line {
  colour: number;
  initials: string;
}

export interface LineSource {
  slug: string;
  created: string;
}

function hash(text: string): number {
  let value = 0x811c9dc5;
  for (let index = 0; index < text.length; index++) {
    value ^= text.charCodeAt(index);
    value = Math.imul(value, 0x01000193);
  }
  return value >>> 0;
}

// assignLines is stable: workstreams claim colours in creation order, each
// starting at a hash of its slug and probing for a free line, so adding a
// newer workstream never recolours an older one.
export function assignLines(workstreams: LineSource[]): Map<string, Line> {
  const ordered = [...workstreams].sort(
    (a, b) =>
      a.created.localeCompare(b.created) || a.slug.localeCompare(b.slug),
  );
  const used = new Set<number>();
  const lines = new Map<string, Line>();
  for (const workstream of ordered) {
    let colour = hash(workstream.slug) % LINE_COUNT;
    if (used.size < LINE_COUNT) {
      while (used.has(colour)) colour = (colour + 1) % LINE_COUNT;
    }
    used.add(colour);
    lines.set(workstream.slug, {
      colour,
      initials: lineInitials(workstream.slug),
    });
  }
  return lines;
}

export function lineInitials(slug: string): string {
  const words = slug.split(/[-_\s]+/).filter(Boolean);
  if (words.length === 0) return "?";
  return words
    .slice(0, 2)
    .map((word) => word[0]?.toUpperCase() ?? "")
    .join("");
}

export interface ProjectLines {
  project: string;
  workstreams: (LineSource & { title: string })[];
}

// linesByProject assigns lines per project so bullets match across views.
export function linesByProject(
  projects: ProjectLines[],
): Map<string, Map<string, Line>> {
  const result = new Map<string, Map<string, Line>>();
  for (const project of projects) {
    result.set(project.project, assignLines(project.workstreams));
  }
  return result;
}

// workstreamTitle names a project's workstream by its title, or by its slug
// when the project lists no such workstream.
export function workstreamTitle(
  workstreams: Map<string, WorkstreamBrief[]>,
  project: string,
  slug: string,
): string {
  return (
    workstreams.get(project)?.find((workstream) => workstream.slug === slug)
      ?.title ?? slug
  );
}

// workstreamTitleKey is what a workstream's title is compared by: its
// words in lower case, every run of spaces as one. It is the server's rule
// (board.WorkstreamTitleKey): a project's workstreams have distinct keys,
// so no two read the same in a picker (FH-163, D76).
export function workstreamTitleKey(title: string): string {
  return title.trim().split(/\s+/).join(" ").toLowerCase();
}

// takenWorkstream is the workstream among a project's that already has
// title, other than except (the slug of a workstream being retitled);
// none for an empty title. The server refuses such a title; this lets a
// form say so before sending it.
export function takenWorkstream(
  workstreams: WorkstreamBrief[],
  title: string,
  except = "",
): WorkstreamBrief | undefined {
  const key = workstreamTitleKey(title);
  if (key === "") return undefined;
  return workstreams.find(
    (workstream) =>
      workstream.slug !== except &&
      workstreamTitleKey(workstream.title) === key,
  );
}
