import { useCallback, useEffect, useState } from "react";

import { TICKET_ID, type TicketRef } from "../api/board";

export type View = "board" | "workstreams" | "table";
export type Scope = { kind: "all" } | { kind: "project"; project: string };

export interface Route {
  scope: Scope;
  view: View;
  ticket?: TicketRef;
}

const views: View[] = ["board", "workstreams", "table"];

// Routes live in the URL hash (#/all/board, #/p/<project>/<view>?t=<ticket id>)
// so reloads and new tabs keep their place without any browser storage.
export function parseRoute(hash: string): Route {
  const [path = "", query = ""] = hash.replace(/^#/, "").split("?");
  const parts = path.split("/").filter(Boolean).map(decode);
  let scope: Scope = { kind: "all" };
  let viewPart: string | undefined;
  if (parts[0] === "p" && parts[1]) {
    scope = { kind: "project", project: parts[1] };
    viewPart = parts[2];
  } else if (parts[0] === "all") {
    viewPart = parts[1];
  }
  const view = views.includes(viewPart as View) ? (viewPart as View) : "board";
  const route: Route = { scope, view };
  const ticket = new URLSearchParams(query).get("t");
  if (ticket && TICKET_ID.test(ticket)) route.ticket = { id: ticket };
  return route;
}

export function formatRoute(route: Route): string {
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
    const onChange = () => setRoute(parseRoute(window.location.hash));
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
