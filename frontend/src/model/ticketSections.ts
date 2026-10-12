// A ticket's sections, in order, with their counts: the card panel's tabs
// today, the full page's outline later (CARD-1, CARD-7, ui-layout.md §3).
import type { TicketDetail } from "../api/board";
import { sessionsOf } from "./runs";

export type SectionId = "ticket" | "edit" | "runs" | "attachments" | "review";

export interface TicketSection {
  id: SectionId;
  title: string;
  // count is how many items the section holds, where it counts any.
  count?: number;
}

// ticketSections lists the sections a ticket offers: Ticket always, Edit
// when it can be edited, Runs (counting sessions, not their subagents),
// Attachments when it has files, and Review when it has review.md.
export function ticketSections(
  detail: Pick<TicketDetail, "runs" | "attachmentFiles" | "review">,
  editable: boolean,
): TicketSection[] {
  const sessions = sessionsOf(detail.runs).length;
  const files = detail.attachmentFiles.length;
  return [
    { id: "ticket", title: "Ticket" },
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
