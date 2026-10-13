// A ticket's sections, in order, with their counts: the card panel's tabs
// (CARD-1) and the full page's sections and outline (CARD-7, ui-layout.md
// §3).
import type { Attachment, Review, TicketDetail } from "../api/board";
import {
  type ImageRef,
  imageRefs,
  linkedFiles,
  type MarkdownSection,
  markdownSections,
  withoutImages,
} from "./markdown";
import { sessionsOf } from "./runs";

export type SectionId =
  | "ticket"
  | "comments"
  | "edit"
  | "runs"
  | "attachments"
  | "review";

export interface TicketSection {
  id: SectionId;
  title: string;
  // count is how many items the section holds, where it counts any.
  count?: number;
}

// ticketSections lists the sections a ticket offers: Ticket always,
// Comments (counting them) when it has any or can be edited, since it then
// holds the comment box (FH-128), Edit when it can be edited, Runs
// (counting sessions, not their subagents), Attachments when it has files,
// and Review when it has review.md.
export function ticketSections(
  detail: Pick<
    TicketDetail,
    "runs" | "attachmentFiles" | "review" | "comments"
  >,
  editable: boolean,
): TicketSection[] {
  const sessions = sessionsOf(detail.runs).length;
  const files = detail.attachmentFiles.length;
  const comments = detail.comments.length;
  return [
    { id: "ticket", title: "Ticket" },
    ...(comments || editable
      ? [
          {
            id: "comments" as const,
            title: "Comments",
            ...(comments ? { count: comments } : {}),
          },
        ]
      : []),
    ...(editable ? [{ id: "edit" as const, title: "Edit" }] : []),
    { id: "runs", title: "Runs", ...(sessions ? { count: sessions } : {}) },
    ...(files
      ? [{ id: "attachments" as const, title: "Attachments", count: files }]
      : []),
    ...(detail.review ? [{ id: "review" as const, title: "Review" }] : []),
  ];
}

// sectionLabel is a section's name with its count, as a tab shows it.
export function sectionLabel(section: TicketSection): string {
  return section.count ? `${section.title} ${section.count}` : section.title;
}

// The full page (CARD-7, ui-layout.md §3 Ticket page) shows the whole
// ticket as one page: its own sections, then its review's, then the files
// the review does not show, then its activity. The outline lists these.

export type PageGroup = "ticket" | "review" | "files" | "activity";

export type PageSectionKind =
  | "repair"
  | "questions"
  | "handoff"
  | "blocked"
  | "criteria"
  | "text"
  | "review"
  | "review-text"
  | "evidence"
  | "verify"
  | "files"
  | "activity";

export interface PageSection {
  // id is unique on the page: the section's element is section-<id>.
  id: string;
  kind: PageSectionKind;
  group: PageGroup;
  title: string;
  // count is what the outline shows beside the title: "d/t" for a
  // checklist, a number for a list.
  count?: string;
  // complete marks a checklist whose items are all done.
  complete?: boolean;
  // empty marks a ticket section the template has that holds nothing; the
  // outline lists it and the page leaves it out.
  empty?: boolean;
  // markdown is a text section's body (the review's lead for "review").
  markdown?: string;
  // heading names the section of the ticket file an in-place edit saves
  // to (FH-66): as written, or the template's name for a missing one.
  // Absent for what is not edited in place: Handoff, the review, Files,
  // Activity, and a repeated heading after its first section.
  heading?: string;
  // template marks a section of the ticket template (criteria,
  // Description, Test plan, Reproduction, Context).
  template?: boolean;
  // images are the Evidence section's screenshots, in order.
  images?: ImageRef[];
}

export type PageInput = Pick<
  TicketDetail,
  | "type"
  | "body"
  | "needsRepair"
  | "questions"
  | "handoff"
  | "blockedBy"
  | "criteriaItems"
  | "review"
  | "attachmentFiles"
>;

// Sections of the ticket file the page shows elsewhere: criteria from the
// parsed checklist, the handoff as its callout, notes in Activity.
const placedElsewhere = ["acceptance criteria", "handoff", "notes"];

// The template's text sections, in the page's order, with their sentence
// case titles (board-format.md, Ticket).
const templateText: {
  id: string;
  heading: string;
  title: string;
  name: string;
}[] = [
  {
    id: "description",
    heading: "description",
    title: "Description",
    name: "Description",
  },
  {
    id: "test-plan",
    heading: "test plan",
    title: "Test plan",
    name: "Test Plan",
  },
  {
    id: "reproduction",
    heading: "reproduction",
    title: "Reproduction",
    name: "Reproduction",
  },
  { id: "context", heading: "context", title: "Context", name: "Context" },
];

// The review template's headings in sentence case (agent-protocol review
// template); any other heading keeps its words.
const reviewTitles: Record<string, string> = {
  summary: "Summary",
  "key files": "Key files",
  evidence: "Evidence",
  "how to verify": "How to verify",
  "risks / things to watch": "Risks / things to watch",
  tests: "Tests",
};

const checklistCount = (items: { done: boolean }[]) => {
  const done = items.filter((item) => item.done).length;
  return { count: `${done}/${items.length}`, complete: done === items.length };
};

const slug = (text: string) =>
  text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "") || "section";

const plain = (heading: string) => heading.trim().toLowerCase();

// pageSections lists the full page's sections in order (FH-65): needs
// repair, questions waiting, Handoff, Blocked by, Acceptance criteria,
// Description, Test plan (or Reproduction), Context and the ticket's other
// sections; then Review, when review.md exists, in its own sections; then
// Files, the attachments neither the review nor the ticket links; then
// Activity, with activity entries.
export function pageSections(
  detail: PageInput,
  activity: number,
): PageSection[] {
  const list: PageSection[] = [];
  const taken = new Set<string>();
  const add = (section: PageSection) => {
    let id = section.id;
    for (let n = 2; taken.has(id); n++) id = `${section.id}-${n}`;
    taken.add(id);
    list.push({ ...section, id });
  };
  const ticket = (section: Omit<PageSection, "group">) =>
    add({ ...section, group: "ticket" });

  if (detail.needsRepair.length > 0)
    ticket({ id: "repair", kind: "repair", title: "Needs repair" });
  if (detail.questions.length > 0)
    ticket({
      id: "questions",
      kind: "questions",
      title:
        detail.questions.length === 1
          ? "Question for you"
          : "Questions for you",
      count: String(detail.questions.length),
    });
  if (detail.handoff)
    ticket({ id: "handoff", kind: "handoff", title: "Handoff" });
  if (detail.blockedBy.length > 0)
    ticket({ id: "blocked", kind: "blocked", title: "Blocked by" });
  const { lead, sections } = markdownSections(detail.body);
  const byHeading = (heading: string) =>
    sections.find((section) => plain(section.title) === heading);
  ticket({
    id: "criteria",
    kind: "criteria",
    title: "Acceptance criteria",
    heading: byHeading("acceptance criteria")?.title ?? "Acceptance Criteria",
    template: true,
    empty: detail.criteriaItems.length === 0,
    ...(detail.criteriaItems.length > 0
      ? checklistCount(detail.criteriaItems)
      : {}),
  });
  const bug = detail.type === "bug";
  for (const text of templateText) {
    const found = byHeading(text.heading);
    // A ticket has a test plan or, as a bug, a reproduction: the one it
    // lacks is not listed.
    if (!found && text.id === "test-plan" && (bug || byHeading("reproduction")))
      continue;
    if (!found && text.id === "reproduction" && !bug) continue;
    const markdown = [text.id === "description" ? lead : "", found?.markdown]
      .filter(Boolean)
      .join("\n\n");
    ticket({
      id: text.id,
      kind: "text",
      title: text.title,
      heading: found?.title ?? text.name,
      template: true,
      markdown,
      empty: !markdown.trim(),
    });
  }
  const known = new Set([
    ...placedElsewhere,
    ...templateText.map((text) => text.heading),
  ]);
  // A repeated heading is edited only in its first section, the one a
  // save by name reaches.
  const named = new Set<string>();
  for (const section of sections) {
    if (known.has(plain(section.title))) continue;
    const first = !named.has(plain(section.title));
    named.add(plain(section.title));
    ticket({
      id: slug(section.title),
      kind: "text",
      title: section.title,
      ...(first ? { heading: section.title } : {}),
      markdown: section.markdown,
      empty: !section.markdown.trim(),
    });
  }

  if (detail.review) {
    const review = reviewParts(detail.review);
    add({
      id: "review",
      kind: "review",
      group: "review",
      title: "Review",
      markdown: review.lead,
    });
    for (const section of review.sections) add(section);
  }

  const files = unreferencedFiles(detail).length;
  if (files > 0)
    add({
      id: "files",
      kind: "files",
      group: "files",
      title: "Files",
      count: String(files),
    });
  add({
    id: "activity",
    kind: "activity",
    group: "activity",
    title: "Activity",
    ...(activity > 0 ? { count: String(activity) } : {}),
  });
  return list;
}

// reviewParts splits review.md into its lead and its own sections, How to
// Verify as the checklist of its steps (REV-3) when it has steps.
function reviewParts(review: Review): {
  lead: string;
  sections: PageSection[];
} {
  const steps = review.steps.length > 0;
  const before = markdownSections(steps ? review.before : review.markdown);
  const after = steps ? markdownSections(review.after).sections : [];
  const section = ({ title, markdown }: MarkdownSection): PageSection => {
    const heading = plain(title);
    const base = {
      id: `review-${slug(title)}`,
      group: "review" as const,
      title: reviewTitles[heading] ?? title,
      markdown,
    };
    if (heading === "evidence") {
      const images = imageRefs(markdown);
      return {
        ...base,
        kind: "evidence",
        markdown: withoutImages(markdown),
        images,
        ...(images.length > 0 ? { count: String(images.length) } : {}),
      };
    }
    return { ...base, kind: "review-text" };
  };
  const verify: PageSection[] = steps
    ? [
        {
          id: "review-how-to-verify",
          kind: "verify",
          group: "review",
          title: "How to verify",
          ...checklistCount(review.steps),
        },
      ]
    : [];
  return {
    lead: before.lead,
    sections: [
      ...before.sections.map(section),
      ...verify,
      ...after.map(section),
    ],
  };
}

// unreferencedFiles are the ticket's attachments that neither review.md
// nor the ticket file links or shows, so each screenshot appears once on
// the page: the rest are under Files.
export function unreferencedFiles(
  detail: Pick<TicketDetail, "body" | "review" | "attachmentFiles">,
): Attachment[] {
  const linked = new Set([
    ...linkedFiles(detail.body),
    ...linkedFiles(detail.review?.markdown ?? ""),
  ]);
  return detail.attachmentFiles.filter((file) => !linked.has(file.file));
}
