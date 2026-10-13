import { type ReactNode, useId } from "react";
import type { TicketDetail, TicketRef } from "../../api/board";
import { panelId, Tabs, tabId } from "../../components/Tabs";
import {
  type SectionId,
  sectionLabel,
  type TicketSection,
} from "../../model/ticketSections";
import { AttachmentDrop } from "./AttachmentDrop";
import { Attachments } from "./Attachments";
import { ReviewTab } from "./ReviewTab";
import { RunsTab } from "./RunsTab";
import { TicketTab } from "./TicketTab";
import type { TicketState } from "./useTicket";

// SectionBody is one of the card panel's tabs for a loaded ticket: Ticket,
// Runs, Attachments (REV-1) or Review (REV-3). The full page shows the
// ticket as one page instead (TicketPage). The panel keeps its Comments
// (FH-128) and Edit (EDIT-6, FH-138) tabs mounted itself (CardPanel).
export function SectionBody({
  section,
  detail,
  ticket,
  keys,
  onOpen,
}: {
  section: Exclude<SectionId, "comments" | "edit">;
  detail: TicketDetail;
  ticket: TicketState;
  keys: Set<string>;
  onOpen(ticket: TicketRef): void;
}) {
  switch (section) {
    case "review":
      return <ReviewTab detail={detail} keys={keys} onVerify={ticket.verify} />;
    case "attachments":
      // Files dropped here, or chosen, are attached (FH-18) when the
      // ticket can be edited.
      return ticket.edit ? (
        <AttachmentDrop upload={ticket.edit.upload}>
          {detail.attachmentFiles.length > 0 ? (
            <Attachments attachments={detail.attachmentFiles} />
          ) : null}
        </AttachmentDrop>
      ) : (
        <Attachments attachments={detail.attachmentFiles} />
      );
    case "runs":
      return <RunsTab detail={detail} onOpen={onOpen} />;
    default:
      return (
        <TicketTab
          detail={detail}
          keys={keys}
          onOpen={onOpen}
          onToggle={ticket.toggle}
          onAnswer={ticket.answer}
        />
      );
  }
}

// SectionTabs shows a ticket's sections as tabs over one tab panel, which
// renders the active section (the card panel's tabs).
export function SectionTabs({
  sections,
  active,
  onSelect,
  tabsClassName,
  bodyClassName,
  children,
}: {
  sections: TicketSection[];
  active: SectionId;
  onSelect(id: SectionId): void;
  tabsClassName: string;
  bodyClassName: string;
  children: ReactNode;
}) {
  const tabsId = useId();
  return (
    <>
      <div className={tabsClassName}>
        <Tabs
          idPrefix={tabsId}
          label="Ticket views"
          selected={active}
          onSelect={(id) => onSelect(id as SectionId)}
          items={sections.map((section) => ({
            id: section.id,
            label: sectionLabel(section),
          }))}
        />
      </div>
      <div
        role="tabpanel"
        // The ARIA tabs pattern makes the panel a tab stop, so a scrolling
        // panel with no controls (Runs) is still reachable.
        // biome-ignore lint/a11y/noNoninteractiveTabindex: see above.
        tabIndex={0}
        id={panelId(tabsId, active)}
        aria-labelledby={tabId(tabsId, active)}
        className={bodyClassName}
      >
        {children}
      </div>
    </>
  );
}

// activeSection is the chosen section, or Ticket when it is not offered.
export function activeSection(
  sections: TicketSection[],
  chosen: SectionId,
): SectionId {
  return sections.some((section) => section.id === chosen) ? chosen : "ticket";
}

// TicketSkeleton holds the place of a ticket while it loads.
export function TicketSkeleton() {
  return (
    <div
      aria-hidden="true"
      data-skeleton="ticket"
      className="flex flex-col gap-3 p-6"
    >
      <div className="h-3 w-24 animate-pulse rounded-card bg-well" />
      <div className="h-6 w-3/4 animate-pulse rounded-card bg-well" />
      <div className="h-3 w-1/2 animate-pulse rounded-card bg-well" />
      <div className="mt-6 h-24 animate-pulse rounded-card bg-well" />
    </div>
  );
}
