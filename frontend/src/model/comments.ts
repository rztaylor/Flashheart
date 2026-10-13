// People's comments on a ticket (STO-8, D47, FH-125): the parts of the
// ticket a comment can quote, and one comment's entry in the activity file
// for the conflict dialog.
import { type PageInput, pageSections } from "./ticketSections";

// QuoteChoice is a part of the ticket a comment can quote: from names where
// it is ("Acceptance criteria 2"), text is what is quoted, label how the
// picker lists it.
export interface QuoteChoice {
  key: string;
  label: string;
  from: string;
  text: string;
}

export interface QuoteGroup {
  label: string;
  choices: QuoteChoice[];
}

// quoteLimit is the most characters a section's quote takes; the server's
// limit is 4 KiB (board.MaxQuote), and a quote is a reminder of the part,
// not a copy of it.
const quoteLimit = 1000;

// cutQuote cuts text to quoteLimit characters, at a line end where one is
// near, ending it with an ellipsis.
function cutQuote(text: string): string {
  const trimmed = text.trim();
  if (trimmed.length <= quoteLimit) return trimmed;
  const head = trimmed.slice(0, quoteLimit);
  const line = head.lastIndexOf("\n");
  return `${(line > quoteLimit / 2 ? head.slice(0, line) : head).trimEnd()}…`;
}

// short keeps a picker's line to one readable line.
const short = (text: string) =>
  text.length > 80 ? `${text.slice(0, 79).trimEnd()}…` : text;

// quoteChoices lists what a comment can quote, in the page's order: the
// ticket's text sections and the review's (with words in them), each
// acceptance criterion, and each How to verify step. Empty groups are left
// out.
export function quoteChoices(detail: PageInput): QuoteGroup[] {
  const sections = pageSections(detail, 0)
    .filter(
      (section) =>
        (section.kind === "text" || section.kind === "review-text") &&
        section.markdown?.trim(),
    )
    .map((section): QuoteChoice => {
      const name =
        section.group === "review" ? `Review: ${section.title}` : section.title;
      return {
        key: `section-${section.id}`,
        label: short(name),
        from: name,
        text: cutQuote(section.markdown ?? ""),
      };
    });
  const numbered = (
    prefix: string,
    key: string,
    items: { text: string }[],
  ): QuoteChoice[] =>
    items.map((item, index) => ({
      key: `${key}-${index + 1}`,
      label: short(`${index + 1}. ${item.text}`),
      from: `${prefix} ${index + 1}`,
      text: cutQuote(item.text),
    }));
  return [
    { label: "Sections", choices: sections },
    {
      label: "Criteria",
      choices: numbered(
        "Acceptance criteria",
        "criterion",
        detail.criteriaItems,
      ),
    },
    {
      label: "How to verify steps",
      choices: numbered("How to verify", "step", detail.review?.steps ?? []),
    },
  ].filter((group) => group.choices.length > 0);
}

// An entry's heading in the activity file (board-format §Activity).
const entryHeading = /^### \S+ · [a-z][a-z0-9-]* · .+ · [0-9A-Za-z_-]+[ \t]*$/;

// commentInFile is one comment's entry in an activity file, from its
// heading to the next entry, or the whole file when it has no such
// comment: the part a conflict dialog compares.
export function commentInFile(file: string, id: string): string {
  const lines = file.split("\n");
  const start = lines.findIndex(
    (line) =>
      entryHeading.test(line) &&
      line.includes(" · comment · ") &&
      line.trimEnd().endsWith(` · ${id}`),
  );
  if (start < 0) return file;
  let end = start + 1;
  while (end < lines.length && !entryHeading.test(lines[end] ?? "")) end++;
  return lines.slice(start, end).join("\n").trimEnd();
}

// DeletableComment is what decides whether a comment can be deleted
// (FH-140): its state as a change request, and where its delivery to a
// session stands, with the run it was sent to (short).
export interface DeletableComment {
  state: string;
  delivery: string;
  sentTo: string;
}

// deleteRefusal says why a comment cannot be deleted, or undefined when it
// can: once a session has read it, and while it is a change request open
// or addressed (the server holds the same rule, board.CanDeleteComment).
export function deleteRefusal(comment: DeletableComment): string | undefined {
  if (comment.delivery === "delivered")
    return `It can't be deleted: ${comment.sentTo || "its session"} has read it.`;
  if (comment.state === "change-requested")
    return "It can't be deleted while the change it asks for is open. Resolve it first.";
  if (comment.state === "addressed")
    return "It can't be deleted while the change request waits for you. Resolve it first.";
  return undefined;
}

// deleteNote says what deleting a comment does besides removing it: one
// sent to a session and still waiting is withdrawn, so the session never
// reads it; history keeps the deletion, never the text.
export function deleteNote(comment: DeletableComment): string {
  const history =
    "The history keeps that you deleted a comment, not what it said.";
  return comment.delivery === "waiting"
    ? `${comment.sentTo || "Its session"} has not taken it yet and never will. ${history}`
    : history;
}
