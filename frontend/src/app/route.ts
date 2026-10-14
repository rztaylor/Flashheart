import { useCallback, useEffect, useRef, useState } from "react";

import { TICKET_ID, type TicketRef } from "../api/board";
import { ticketPageHref, workstreamPageHref } from "../model/markdown";
import { guarded, leave } from "../state/leaveGuard";

// archive is reached from the page header, not the band (EDIT-8); ticket
// is a ticket's full page (CARD-7), reached from its id, and workstream a
// workstream's page (FH-139), reached from its route card.
export type View =
  | "board"
  | "overview"
  | "workstreams"
  | "table"
  | "archive"
  | "ticket"
  | "workstream";
export type Scope = { kind: "all" } | { kind: "project"; project: string };

// WorkstreamRef names a workstream: its project and slug.
export interface WorkstreamRef {
  project: string;
  slug: string;
}

export interface Route {
  scope: Scope;
  view: View;
  ticket?: TicketRef;
  workstream?: WorkstreamRef;
}

// SLUG is a workstream slug (board-format §Workstream).
const SLUG = /^[a-z0-9][a-z0-9._-]{0,99}$/;

const views: View[] = ["board", "overview", "workstreams", "table", "archive"];

// The Overview replaced the Agents view (D32); its old routes land there.
const aliases: Record<string, View> = { agents: "overview" };

// Routes live in the URL hash (#/all/board, #/p/<project>/<view>?t=<ticket
// id>, #/ticket/<ticket id>, #/workstream/<project>/<slug>) so reloads and
// new tabs keep their place without any browser storage.
export function parseRoute(hash: string): Route {
  const [path = "", query = ""] = hash.replace(/^#/, "").split("?");
  const parts = path.split("/").filter(Boolean).map(decode);
  if (parts[0] === "ticket") {
    const id = parts[1] ?? "";
    return TICKET_ID.test(id)
      ? { scope: { kind: "all" }, view: "ticket", ticket: { id } }
      : { scope: { kind: "all" }, view: "board" };
  }
  if (parts[0] === "workstream") {
    const [project = "", slug = ""] = parts.slice(1);
    return project && project !== "." && project !== ".." && SLUG.test(slug)
      ? {
          scope: { kind: "all" },
          view: "workstream",
          workstream: { project, slug },
        }
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
  const view = views.includes(named as View) ? (named as View) : "board";
  const route: Route = { scope, view };
  const ticket = new URLSearchParams(query).get("t");
  if (ticket && TICKET_ID.test(ticket)) route.ticket = { id: ticket };
  return route;
}

export function formatRoute(route: Route): string {
  if (route.view === "ticket" && route.ticket)
    return ticketPageHref(route.ticket.id);
  if (route.view === "workstream" && route.workstream)
    return workstreamPageHref(route.workstream);
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

// useRoute is the route in the URL hash and the way to change it. While an
// edit is unsaved, leaving asks first (state/leaveGuard, FH-66): its own
// navigation waits, and a link or Back that already changed the hash is
// put back until the person agrees to drop the edit.
// Navigate changes the route; replace puts the new route in place of the
// current history entry (a renamed workstream's page, FH-167), so Back
// does not return to an address that is gone.
export type Navigate = (next: Route, options?: { replace?: boolean }) => void;

export function useRoute(): [Route, Navigate] {
  const [route, setRoute] = useState(() => parseRoute(window.location.hash));
  // shown is the hash of the route on screen, to put back a held change.
  const shown = useRef(window.location.hash);
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
    shown.current = window.location.hash;
    const onChange = () => {
      const target = window.location.hash;
      if (target !== shown.current && guarded()) {
        window.history.replaceState(null, "", shown.current || "#");
        leave(() => {
          window.location.hash = target;
        });
        return;
      }
      canonical();
      shown.current = window.location.hash;
      setRoute(parseRoute(window.location.hash));
    };
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  const navigate = useCallback<Navigate>((next, options) => {
    leave(() => {
      const hash = formatRoute(next);
      shown.current = hash;
      // Replacing fires no hashchange; the route is set here either way.
      if (options?.replace) window.history.replaceState(null, "", hash);
      else if (window.location.hash !== hash) window.location.hash = hash;
      setRoute(next);
    });
  }, []);
  return [route, navigate];
}
