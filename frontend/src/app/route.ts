import { useCallback, useEffect, useState } from "react";

import { TICKET_ID, type TicketRef } from "../api/board";
import { ticketPageHref } from "../model/markdown";

// archive is reached from the page header, not the band (EDIT-8); ticket
// is a ticket's full page (CARD-7), reached from its id.
export type View =
  | "board"
  | "overview"
  | "workstreams"
  | "table"
  | "archive"
  | "ticket";
export type Scope = { kind: "all" } | { kind: "project"; project: string };

export interface Route {
  scope: Scope;
  view: View;
  ticket?: TicketRef;
}

const views: View[] = ["board", "overview", "workstreams", "table", "archive"];

// The Overview replaced the Agents view (D32); its old routes land there.
const aliases: Record<string, View> = { agents: "overview" };

// Routes live in the URL hash (#/all/board, #/p/<project>/<view>?t=<ticket
// id>, #/ticket/<ticket id>) so reloads and new tabs keep their place
// without any browser storage.
export function parseRoute(hash: string): Route {
  const [path = "", query = ""] = hash.replace(/^#/, "").split("?");
  const parts = path.split("/").filter(Boolean).map(decode);
  if (parts[0] === "ticket") {
    const id = parts[1] ?? "";
    return TICKET_ID.test(id)
      ? { scope: { kind: "all" }, view: "ticket", ticket: { id } }
      : { scope: { kind: "all" }, view: "board" };
  }
  let scope: Scope = { kind: "all" };
  let viewPart: string | undefined;
  if (parts[0] === "p" && parts[1]) {
    scope = { kind: "project", project: parts[1] };
    viewPart = parts[2];
  } else if (parts[0] === "all") {
    viewPart = parts[1];
  }
  const named = aliases[viewPart ?? ""] ?? viewPart;
  const view =
    views.includes(named as View) && named !== "ticket"
      ? (named as View)
      : "board";
  const route: Route = { scope, view };
  const ticket = new URLSearchParams(query).get("t");
  if (ticket && TICKET_ID.test(ticket)) route.ticket = { id: ticket };
  return route;
}

export function formatRoute(route: Route): string {
  if (route.view === "ticket" && route.ticket)
    return ticketPageHref(route.ticket.id);
  const base =
    route.scope.kind === "all"
      ? "#/all"
      : `#/p/${encodeURIComponent(route.scope.project)}`;
  const ticket = route.ticket
    ? `?t=${encodeURIComponent(route.ticket.id)}`
    : "";
  return `${base}/${route.view}${ticket}`;
}

function decode(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export function useRoute(): [Route, (next: Route) => void] {
  const [route, setRoute] = useState(() => parseRoute(window.location.hash));
  useEffect(() => {
    // An old route (#/all/agents) is rewritten to the view it lands on.
    const canonical = () => {
      const [path = ""] = window.location.hash.split("?");
      if (path.endsWith("/agents"))
        window.history.replaceState(
          null,
          "",
          formatRoute(parseRoute(window.location.hash)),
        );
    };
    canonical();
    const onChange = () => {
      canonical();
      setRoute(parseRoute(window.location.hash));
    };
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  const navigate = useCallback((next: Route) => {
    const hash = formatRoute(next);
    if (window.location.hash !== hash) window.location.hash = hash;
    setRoute(next);
  }, []);
  return [route, navigate];
}
