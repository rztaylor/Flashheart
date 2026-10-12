import { useCallback, useRef, useState } from "react";

import type { TicketRef } from "../api/board";
import type { PanelTab } from "../features/card/CardPanel";
import type { Route } from "./route";

// useTicketPanel opens a ticket in the card panel, on a given tab when asked
// (the Overview's Review results), and closes it, returning focus to what
// opened it.
export function useTicketPanel(route: Route, navigate: (route: Route) => void) {
  const [panelTab, setPanelTab] = useState<{ id: string; tab: PanelTab }>();
  const opener = useRef<HTMLElement | null>(null);
  const openTicket = (ticket: TicketRef, tab?: PanelTab) => {
    setPanelTab(tab ? { id: ticket.id, tab } : undefined);
    // Remember what opened the panel (a card, row or station) so Escape can
    // return focus there; links inside the panel keep the original opener.
    const active = document.activeElement;
    if (active instanceof HTMLElement && !active.closest("aside, dialog")) {
      opener.current = active;
    }
    navigate({ ...route, ticket });
  };
  const closeTicket = useCallback(() => {
    navigate({ ...route, ticket: undefined });
    const element = opener.current;
    const id = route.ticket?.id;
    opener.current = null;
    window.setTimeout(() => {
      // Return to what opened the panel, or else to the ticket's own card.
      if (element?.isConnected) element.focus();
      else if (id)
        document
          .querySelector<HTMLElement>(`[data-ticket="${CSS.escape(id)}"]`)
          ?.focus();
    }, 0);
  }, [navigate, route]);
  return {
    openTicket,
    closeTicket,
    // initialTab is the tab the open ticket was asked to open on.
    initialTab:
      panelTab && panelTab.id === route.ticket?.id ? panelTab.tab : undefined,
  };
}
