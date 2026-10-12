import { type ReactNode, useState } from "react";
import type { TicketDetail, TicketRef, WorkstreamBrief } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { Icon } from "../../components/Icon";
import { LoadError } from "../../components/LoadError";
import { SidePanel } from "../../components/SidePanel";
import { StaleNote } from "../../components/StaleNote";
import { TicketLink } from "../../components/TicketLink";
import { type Line, workstreamTitle } from "../../model/lines";
import type { Step } from "../../model/order";
import { type SectionId, ticketSections } from "../../model/ticketSections";
import type { Editing } from "../editing/useEditing";
import {
  TicketActions,
  TicketIdentity,
  TicketProperties,
} from "./TicketHeader";
import {
  activeSection,
  SectionBody,
  SectionTabs,
  TicketSkeleton,
} from "./TicketSections";
import { useTicket } from "./useTicket";

export interface CardPanelProps {
  ticket: TicketRef;
  fetcher: AuthenticatedFetch;
  lines: Map<string, Map<string, Line>>;
  // workstreams are each project's workstreams, by project name.
  workstreams: Map<string, WorkstreamBrief[]>;
  // Project keys, so ticket ids in markdown become links (KEY-3).
  keys: Set<string>;
  onOpen(ticket: TicketRef): void;
  onClose(): void;
  // revision reloads the ticket when the board changes (LIFE-3).
  revision: number;
  // editing enables moves, archiving and the Edit tab; absent when the
  // server is read-only.
  editing?: Editing;
  // onStep moves the ticket within its column (EDIT-9); absent when the
  // board does not show it.
  onStep?: (step: Step) => void;
  // answers: the server records answers to agents' questions (RUN-8).
  answers?: boolean;
  // initialTab is the tab the ticket opens on (the Overview's Review
  // results opens Review); the Ticket tab when absent or not offered.
  initialTab?: PanelTab;
}

export type PanelTab = SectionId;

// CardPanel shows one ticket beside the board (CARD-1), with a way to its
// full page in a new tab (CARD-7): its header, which moves and archives the
// ticket (EDIT-1, EDIT-8), over its sections as tabs that scroll under it.
export function CardPanel({
  ticket: ref,
  fetcher,
  lines,
  workstreams,
  keys,
  onOpen,
  onClose,
  revision,
  editing,
  onStep,
  answers = false,
  initialTab = "ticket",
}: CardPanelProps) {
  const ticket = useTicket({
    fetcher,
    id: ref.id,
    revision,
    editing,
    answers,
  });
  const [tab, setTab] = useState<SectionId>(initialTab);
  const { detail, resource } = ticket;
  const sections = detail ? ticketSections(detail, ticket.editable) : [];
  const active = activeSection(sections, tab);
  return (
    <SidePanel
      label={`Ticket ${ref.id}`}
      onClose={onClose}
      actions={
        <TicketLink
          id={ref.id}
          label="Open full page in a new tab"
          className="grid size-9 place-items-center rounded-full text-ink-muted transition-colors hover:bg-well hover:text-ink"
        >
          <Icon name="expand" />
        </TicketLink>
      }
    >
      {resource.status === "loading" ? <TicketSkeleton /> : null}
      {resource.status === "error" ? (
        <LoadError
          what="This ticket could not be loaded"
          error={resource.error}
          onRetry={resource.reload}
          className="p-6"
        />
      ) : null}
      {detail ? (
        <>
          <PanelHeader
            detail={detail}
            line={lines.get(detail.project)?.get(detail.workstream)}
            workstreamTitle={workstreamTitle(
              workstreams,
              detail.project,
              detail.workstream,
            )}
            actions={
              editing && ticket.editable ? (
                <TicketActions
                  detail={detail}
                  onMove={(to) => void editing.move(detail, to)}
                  onStep={onStep}
                  onArchive={() => {
                    void editing.archive(detail);
                    onClose();
                  }}
                />
              ) : null
            }
          />
          {resource.error ? (
            <StaleNote error={resource.error} className="px-6 pb-3" />
          ) : null}
          <SectionTabs
            sections={sections}
            active={active}
            onSelect={setTab}
            tabsClassName="px-6"
            bodyClassName="relative min-h-0 flex-1 overflow-y-auto px-6 pt-5 pb-10"
          >
            <SectionBody
              section={active}
              detail={detail}
              ticket={ticket}
              fetcher={fetcher}
              keys={keys}
              onOpen={onOpen}
              workstreams={workstreams}
            />
          </SectionTabs>
        </>
      ) : null}
    </SidePanel>
  );
}

// PanelHeader is the panel's header (ui-layout.md §3): the ticket's
// identity, its id linking to the full page, its properties and actions.
export function PanelHeader({
  detail,
  line,
  workstreamTitle,
  actions,
}: {
  detail: TicketDetail;
  line?: Line;
  workstreamTitle: string;
  actions?: ReactNode;
}) {
  return (
    <header className="px-6 pt-5 pb-4 pr-24">
      <TicketIdentity detail={detail} level={2} />
      <TicketProperties
        detail={detail}
        line={line}
        workstreamTitle={workstreamTitle}
      />
      {actions}
    </header>
  );
}
