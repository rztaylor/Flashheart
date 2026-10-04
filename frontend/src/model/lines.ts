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
