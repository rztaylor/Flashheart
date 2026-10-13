// A ticket section's text as the in-place editor edits it (FH-66): read
// from the ticket body exactly as written, compared the way the server
// compares it (board.NewSectionChange: blank lines around it and trailing
// whitespace do not count), and the acceptance criteria as a list.

const fence = /^ {0,3}(```|~~~)/;
const heading = /^ {0,3}(#{1,6})[ \t]+(.*?)[ \t]*#*[ \t]*$/;

// sectionText drops the blank lines before text and the whitespace after
// it: two texts that differ only there save the same.
export function sectionText(text: string): string {
  const lines = text.replace(/\r\n/g, "\n").replace(/\s+$/, "").split("\n");
  while (lines.length > 0 && !lines[0]?.trim()) lines.shift();
  return lines.join("\n");
}

// sameSection reports two section texts that save the same.
export const sameSection = (a: string, b: string) =>
  sectionText(a) === sectionText(b);

// sectionSource is the text of body's first level-2 section named heading
// (ignoring case), outside code fences, as written: what its editor opens
// with. A missing section reads as "".
export function sectionSource(body: string, name: string): string {
  const want = name.trim().toLowerCase();
  const lines: string[] = [];
  let inFence = false;
  let found = false;
  for (const line of body.replace(/\r\n/g, "\n").split("\n")) {
    const fenceLine = fence.test(line);
    const match = !inFence && !fenceLine ? heading.exec(line) : null;
    if (fenceLine) inFence = !inFence;
    const level = match?.[1]?.length ?? 0;
    if (found && level > 0 && level <= 2) break;
    if (found) lines.push(line);
    else if (level === 2 && match?.[2]?.trim().toLowerCase() === want)
      found = true;
  }
  return sectionText(lines.join("\n"));
}

// titleLine is a ticket file's title line ("# Title"), outside code
// fences: the part a title edit shows on a conflict.
export function titleLine(file: string): string {
  let inFence = false;
  for (const line of file.replace(/\r\n/g, "\n").split("\n")) {
    if (fence.test(line)) inFence = !inFence;
    else if (!inFence && heading.exec(line)?.[1] === "#") return line;
  }
  return "";
}

// fieldLines is a frontmatter field as written, with a block list's items:
// the part a property edit shows on a conflict.
export function fieldLines(file: string, key: string): string {
  const lines = file.replace(/\r\n/g, "\n").split("\n");
  if (lines[0] !== "---") return "";
  const out: string[] = [];
  for (const line of lines.slice(1)) {
    if (line === "---") break;
    if (out.length > 0 && /^\s+\S/.test(line)) out.push(line);
    else if (out.length > 0) break;
    else if (line.startsWith(`${key}:`)) out.push(line);
  }
  return out.join("\n");
}

export interface Criterion {
  text: string;
  done: boolean;
}

const taskItem = /^[-*+] \[([ xX])\] (.*?)\s*$/;

// criteriaOf reads an acceptance criteria section that is a plain task
// list (blank lines aside) as its criteria, in order; a section with any
// other text (prose, nested items) is undefined, edited as markdown.
export function criteriaOf(text: string): Criterion[] | undefined {
  const criteria: Criterion[] = [];
  for (const line of sectionText(text).split("\n")) {
    if (!line.trim()) continue;
    const match = taskItem.exec(line);
    if (!match) return undefined;
    criteria.push({ text: match[2] ?? "", done: match[1] !== " " });
  }
  return criteria;
}

// criteriaText writes criteria as the section's task list, one line each,
// dropping empty ones and joining any line breaks in a criterion's text.
export function criteriaText(criteria: Criterion[]): string {
  return criteria
    .map((item) => ({ ...item, text: item.text.replace(/\s+/g, " ").trim() }))
    .filter((item) => item.text)
    .map((item) => `- [${item.done ? "x" : " "}] ${item.text}`)
    .join("\n");
}
