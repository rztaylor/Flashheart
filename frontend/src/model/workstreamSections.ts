// A workstream's sections as its page shows them (FH-139, ui-layout.md
// §4): the file's level-2 sections in order, and each template section the
// file lacks, empty, where the server would add it (board-format
// §Workstream; board.NewWorkstreamSectionChange).
import { markdownSections } from "./markdown";

export interface WorkstreamSection {
  // title is the section's name as shown: a template section's in the
  // template's spelling.
  title: string;
  // heading is the name a save sends, as written in the file.
  heading: string;
  markdown: string;
  // empty: the section has no text (or is not in the file).
  empty: boolean;
  // template: one of the workstream template's sections.
  template: boolean;
}

export const WORKSTREAM_TEMPLATE = [
  "Goal",
  "Scope",
  "Success Criteria",
  "Notes",
] as const;

const templateName = (name: string) =>
  WORKSTREAM_TEMPLATE.find(
    (section) => section.toLowerCase() === name.trim().toLowerCase(),
  );

export function workstreamSections(body: string): WorkstreamSection[] {
  const sections: WorkstreamSection[] = markdownSections(body).sections.map(
    ({ title, markdown }) => {
      const template = templateName(title);
      return {
        title: template ?? title,
        heading: title,
        markdown,
        empty: !markdown,
        template: !!template,
      };
    },
  );
  // A missing template section goes above the first later template
  // section in the file; one of the file's own goes above Notes.
  WORKSTREAM_TEMPLATE.forEach((name, index) => {
    if (sections.some((section) => section.title === name)) return;
    const later = WORKSTREAM_TEMPLATE.slice(index + 1);
    const at = sections.findIndex(
      (section) => section.template && later.includes(section.title as never),
    );
    const missing = {
      title: name,
      heading: name,
      markdown: "",
      empty: true,
      template: true,
    };
    if (at < 0) sections.push(missing);
    else sections.splice(at, 0, missing);
  });
  return sections;
}
