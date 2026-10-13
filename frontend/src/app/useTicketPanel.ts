import { useCallback, useRef, useState } from "react";

import type { TicketRef } from "../api/board";
import type { PanelTab } from "../features/card/CardPanel";
import { leave } from "../state/leaveGuard";
import type { Route } from "./route";

// useTicketPanel opens a ticket in the card panel, on a given tab when asked
// (the Overview's Review results), and closes it, returning focus to what
// opened it.
export function useTicketPanel(route: Route, navigate: (route: Route) => void) {
  const [panelTab, setPanelTab] = useState<{ id: string; tab: PanelTab }>();
  const opener = useRef<HTMLElement | null>(null);
  const openTicket = (ticket: TicketRef, tab?: PanelTab) => {
    // Remember what opened the panel (a card, row or station) so Escape can
    // return focus there; links inside the panel keep the original opener.
    // While unsaved work in the panel asks first (FH-128), nothing changes
    // until the person drops it.
    const active = document.activeElement;
    leave(() => {
      setPanelTab(tab ? { id: ticket.id, tab } : undefined);
      if (active instanceof HTMLElement && !active.closest("aside, dialog")) {
        opener.current = active;
      }
      navigate({ ...route, ticket });
    });
  };
  // closeTicket waits while unsaved work in the panel asks first (a comment
  // being written, FH-128), so focus moves only once the panel closes.
  const closeTicket = useCallback(() => {
    leave(() => {
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
    });
  }, [navigate, route]);
  return {
    openTicket,
    closeTicket,
    // initialTab is the tab the open ticket was asked to open on.
    initialTab:
      panelTab && panelTab.id === route.ticket?.id ? panelTab.tab : undefined,
  };
}
