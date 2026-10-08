import { useCallback, useEffect, useState } from "react";

import type { TicketDetail } from "../../api/board";
import { Icon } from "../../components/Icon";
import { TicketLinksInPlace } from "../../components/TicketLink";
import { type CardPanelProps, TicketView } from "./CardPanel";

interface TicketPageProps extends Omit<CardPanelProps, "onStep"> {
  // boardLink is the ticket open beside its project's board, and the
  // project's name.
  boardLink(project: string): { href: string; label: string };
}

// TicketPage is a ticket's full page (CARD-7), reached from its id: the
// card panel's header, tabs and content at reading width, with room for
// screenshots and, later, editing. Ticket links on it stay in this tab. The
// browser tab is named after the ticket so several open pages stay apart.
export function TicketPage({ boardLink, ...props }: TicketPageProps) {
  const [shown, setShown] = useState<TicketDetail>();
  const onDetail = useCallback((detail: TicketDetail) => setShown(detail), []);
  const board = shown ? boardLink(shown.project) : undefined;
  const title = shown ? `${shown.id} · ${shown.title}` : props.ticket.id;
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
        <TicketView {...props} layout="page" onDetail={onDetail} />
      </div>
    </TicketLinksInPlace>
  );
}
