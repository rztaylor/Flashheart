import { useEffect, useState } from "react";

import { Icon } from "../../components/Icon";
import { LoadError } from "../../components/LoadError";
import { StaleNote } from "../../components/StaleNote";
import { TicketLinksInPlace } from "../../components/TicketLink";
import { workstreamTitle } from "../../model/lines";
import { type SectionId, ticketSections } from "../../model/ticketSections";
import type { CardPanelProps } from "./CardPanel";
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

interface TicketPageProps extends Omit<CardPanelProps, "onStep"> {
  // boardLink is the ticket open beside its project's board, and the
  // project's name.
  boardLink(project: string): { href: string; label: string };
}

// TicketPage is a ticket's full page (CARD-7), reached from its id: the
// card panel's header, sections and content at reading width, with room
// for screenshots and, later, editing. The title is the page's h1, so each
// section sits under an h2 naming it, as the panel's sit under its h2.
// Ticket links on it stay in this tab. The browser tab is named after the
// ticket so several open pages stay apart.
export function TicketPage({
  boardLink,
  ticket: ref,
  fetcher,
  lines,
  workstreams,
  keys,
  onOpen,
  onClose,
  revision,
  editing,
  answers = false,
  initialTab = "ticket",
}: TicketPageProps) {
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
  const board = detail ? boardLink(detail.project) : undefined;
  const title = detail ? `${detail.id} · ${detail.title}` : ref.id;
  useEffect(() => {
    const before = document.title;
    document.title = `${title} · Flashheart`;
    return () => {
      document.title = before;
    };
  }, [title]);
  return (
    <TicketLinksInPlace>
      <div className="mx-auto w-full max-w-5xl px-4 pt-4 md:px-8">
        <nav aria-label="Ticket" className="flex h-9 items-center text-sm">
          {board ? (
            <a
              href={board.href}
              className="inline-flex items-center gap-1.5 rounded-control font-medium text-ink-muted hover:text-ink hover:underline"
            >
              <Icon name="board" size={14} />
              Show on the {board.label} board
            </a>
          ) : null}
        </nav>
        {resource.status === "loading" ? <TicketSkeleton /> : null}
        {resource.status === "error" ? (
          <>
            {/* The page keeps its heading while the ticket cannot load. */}
            <h1 className="sr-only">{ref.id}</h1>
            <LoadError
              what="This ticket could not be loaded"
              error={resource.error}
              onRetry={resource.reload}
              className="py-6"
            />
          </>
        ) : null}
        {detail ? (
          <>
            <header className="pt-2 pb-4">
              <TicketIdentity detail={detail} level={1} />
              <TicketProperties
                detail={detail}
                line={lines.get(detail.project)?.get(detail.workstream)}
                workstreamTitle={workstreamTitle(
                  workstreams,
                  detail.project,
                  detail.workstream,
                )}
              />
              {editing && ticket.editable ? (
                <TicketActions
                  detail={detail}
                  onMove={(to) => void editing.move(detail, to)}
                  onArchive={() => {
                    void editing.archive(detail);
                    onClose();
                  }}
                />
              ) : null}
            </header>
            {resource.error ? (
              <StaleNote error={resource.error} className="pb-3" />
            ) : null}
            <SectionTabs
              sections={sections}
              active={active}
              onSelect={setTab}
              tabsClassName=""
              bodyClassName="pt-6 pb-16"
              heading
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
      </div>
    </TicketLinksInPlace>
  );
}
