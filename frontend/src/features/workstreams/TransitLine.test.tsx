import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Workstream, WorkstreamTicket } from "../../api/board";
import { TransitLine } from "./TransitLine";

// Station states of docs/dev/specs/ui-layout.md §4: a check only for a
// finished ticket, review served but unchecked, a next stop for every ticket
// that can start, held rings for tickets held from outside the line, and a
// dashed missing station; every station names its column in a pill.
const ticket = (
  id: string,
  column: WorkstreamTicket["column"],
  extra: Partial<WorkstreamTicket> = {},
): WorkstreamTicket => ({
  id,
  title: `Ticket ${id}`,
  column,
  blocked: false,
  held: false,
  missing: false,
  dependsOn: [],
  outside: [],
  ...extra,
});

const workstream = (tickets: WorkstreamTicket[]): Workstream => ({
  slug: "board-core",
  title: "Board core",
  status: "active",
  suspended: false,
  declaredStatus: "active",
  priority: "high",
  created: "2026-10-01",
  done: 0,
  total: tickets.length,
  next: "",
  blockedBy: [],
  dependsOnWorkstreams: [],
  tags: [],
  needsRepair: [],
  warnings: [],
  tickets,
});

const render = (tickets: WorkstreamTicket[]) =>
  renderToStaticMarkup(
    <TransitLine
      workstream={workstream(tickets)}
      line={{ colour: 1, initials: "BC" }}
      onOpen={() => undefined}
    />,
  );

const stationIn = (markup: string, id: string) => {
  const at = markup.indexOf(`data-station="${id}"`);
  const next = markup.indexOf("data-station=", at + 1);
  return markup.slice(at, next < 0 ? undefined : next);
};

// FH-1 done → FH-2 review → FH-3 and FH-4 in parallel → FH-5 needs both.
const graph = render([
  ticket("FH-1", "done"),
  ticket("FH-2", "review", { dependsOn: ["FH-1"] }),
  ticket("FH-3", "in-progress", { dependsOn: ["FH-2"] }),
  ticket("FH-4", "backlog", {
    dependsOn: ["FH-2"],
    blocked: true,
    held: true,
    outside: ["NG-7"],
  }),
  ticket("FH-5", "backlog", { dependsOn: ["FH-3", "FH-4"], blocked: true }),
  ticket("FH-9", "", { missing: true }),
  ticket("FH-6", "up-next"),
]);
const station = (id: string) => stationIn(graph, id);

describe("TransitLine stations", () => {
  it("marks each station by its real state", () => {
    expect(station("FH-1")).toContain('data-station-state="done"');
    expect(station("FH-2")).toContain('data-station-state="served"');
    expect(station("FH-3")).toContain('data-station-state="next"');
    expect(station("FH-4")).toContain('data-station-state="held"');
    expect(station("FH-5")).toContain('data-station-state="ahead"');
    expect(station("FH-9")).toContain('data-station-state="missing"');
  });

  it("checks only a finished station", () => {
    const check = 'd="M5 12.5l4.5 4.5L19 7.5"';
    expect(station("FH-1")).toContain(check);
    expect(station("FH-2")).not.toContain(check);
  });

  it("names each station's column in a status pill", () => {
    expect(station("FH-1")).toContain('data-tone="done"');
    expect(station("FH-2")).toContain("Ready to review");
    expect(station("FH-5")).toContain('data-tone="neutral"');
    expect(station("FH-9")).toContain("Does not exist");
  });

  it("says in each station's name what it needs", () => {
    expect(station("FH-5")).toContain("needs Ticket FH-3 and Ticket FH-4");
    expect(station("FH-3")).toContain("next stop");
  });
});

describe("TransitLine track", () => {
  it("draws one track per dependency on the line", () => {
    // FH-1→2, FH-2→3, FH-2→4, FH-3→5, FH-4→5.
    expect(graph.match(/data-track=/g)).toHaveLength(5);
  });

  it("styles track by the station it leads into", () => {
    expect(graph).toContain('data-track="served"');
    expect(graph).toContain('data-track="ahead"');
    // FH-4 is held from outside the line, so its track is suspended.
    expect(graph).toContain('data-track="suspended"');
  });

  it("names a dependency outside the line above its station", () => {
    expect(graph).toContain("Needs NG-7");
  });

  it("puts tickets with no links on the line below the graph", () => {
    expect(graph).toContain(
      "Board core stations with no dependencies on the line",
    );
    expect(graph.indexOf('data-station="FH-6"')).toBeGreaterThan(
      graph.indexOf('data-station="FH-5"'),
    );
    expect(graph).not.toContain("No dependencies on this line");
    expect(graph).toContain("<hr");
  });
});

describe("TransitLine with no links", () => {
  const markup = render([ticket("FH-1", "done"), ticket("FH-2", "backlog")]);

  it("draws stations without track or a separator", () => {
    expect(markup).not.toContain("data-track");
    expect(markup).not.toContain("<hr");
    expect(stationIn(markup, "FH-2")).toContain('data-station-state="next"');
  });
});
