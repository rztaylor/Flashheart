import { type ReactNode, useId } from "react";
import type { TicketDetail, TicketRef, WorkstreamBrief } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { panelId, Tabs, tabId } from "../../components/Tabs";
import {
  type SectionId,
  sectionLabel,
  type TicketSection,
} from "../../model/ticketSections";
import { Attachments } from "./Attachments";
import { EditTab } from "./EditTab";
import { ReviewTab } from "./ReviewTab";
import { RunsTab } from "./RunsTab";
import { TicketTab } from "./TicketTab";
import type { TicketState } from "./useTicket";

// SectionBody is one of the card panel's tabs for a loaded ticket: Ticket,
// Edit (EDIT-6), Runs, Attachments (REV-1) or Review (REV-3). The full page
// shows the ticket as one page instead (TicketPage). The panel keeps its
// Comments tab mounted itself (CardPanel, FH-128).
export function SectionBody({
  section,
  detail,
  ticket,
  fetcher,
  keys,
  onOpen,
  workstreams,
}: {
  section: Exclude<SectionId, "comments">;
  detail: TicketDetail;
  ticket: TicketState;
  fetcher: AuthenticatedFetch;
  keys: Set<string>;
  onOpen(ticket: TicketRef): void;
  workstreams: Map<string, WorkstreamBrief[]>;
}) {
  switch (section) {
    case "review":
      return <ReviewTab detail={detail} keys={keys} onVerify={ticket.verify} />;
    case "attachments":
      return <Attachments attachments={detail.attachmentFiles} />;
    case "runs":
      return <RunsTab detail={detail} onOpen={onOpen} />;
    case "edit":
      return (
        <EditTab
          detail={detail}
          fetcher={fetcher}
          workstreams={workstreams.get(detail.project) ?? []}
          onSaved={ticket.saved}
        />
      );
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
