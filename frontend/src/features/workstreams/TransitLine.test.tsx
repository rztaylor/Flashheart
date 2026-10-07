import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Workstream, WorkstreamTicket } from "../../api/board";
import { TransitLine } from "./TransitLine";

// Station states of docs/dev/specs/ui-layout.md §4: a check only for a
// finished ticket, review served but unchecked, one next stop, rings ahead
// and a dashed missing station; every station names its column in a pill.
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
  ...extra,
});

const workstream: Workstream = {
  slug: "board-core",
  title: "Board core",
  status: "active",
  suspended: false,
  ordered: true,
  declaredStatus: "active",
  priority: "high",
  created: "2026-10-01",
  done: 2,
  total: 5,
  next: "FH-3",
  blockedBy: [],
  dependsOnWorkstreams: [],
  tags: [],
  needsRepair: [],
  warnings: [],
  tickets: [
    ticket("FH-1", "done"),
    ticket("FH-2", "review"),
    ticket("FH-3", "in-progress", { blocked: true }),
    ticket("FH-4", "backlog"),
    ticket("FH-9", "", { missing: true }),
  ],
};

const markup = renderToStaticMarkup(
  <TransitLine
    workstream={workstream}
    line={{ colour: 1, initials: "BC" }}
    onOpen={() => undefined}
  />,
);

const station = (id: string) => {
  const at = markup.indexOf(`data-station="${id}"`);
  const next = markup.indexOf("data-station=", at + 1);
  return markup.slice(at, next < 0 ? undefined : next);
};

describe("TransitLine stations", () => {
  it("marks each station by its real state", () => {
    expect(station("FH-1")).toContain('data-station-state="done"');
    expect(station("FH-2")).toContain('data-station-state="served"');
    expect(station("FH-3")).toContain('data-station-state="next"');
    expect(station("FH-4")).toContain('data-station-state="ahead"');
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
    expect(station("FH-4")).toContain('data-tone="neutral"');
    expect(station("FH-9")).toContain("Does not exist");
  });

  it("flags a blocked next stop with the blocked pill", () => {
    expect(markup).toContain('data-tone="blocked"');
  });
});

// An unordered workstream (an epic) has no sequence to draw: no track joins
// its stations, there is no next stop, and a blocked station is held.
describe("TransitLine for an unordered workstream", () => {
  const epic = renderToStaticMarkup(
    <TransitLine
      workstream={{
        ...workstream,
        ordered: false,
        next: "FH-4",
        tickets: [
          ticket("FH-1", "done"),
          ticket("FH-3", "in-progress", { blocked: true }),
          ticket("FH-4", "backlog"),
        ],
      }}
      line={{ colour: 1, initials: "BC" }}
      onOpen={() => undefined}
    />,
  );
  const at = (id: string) => {
    const from = epic.indexOf(`data-station="${id}"`);
    const next = epic.indexOf("data-station=", from + 1);
    return epic.slice(from, next < 0 ? undefined : next);
  };

  it("draws stations without track or a next stop", () => {
    expect(epic).toContain('data-layout="any-order"');
    expect(epic).toContain("stations, in any order");
    expect(epic).not.toContain("data-track");
    expect(epic).not.toContain('data-station-state="next"');
    expect(at("FH-4")).toContain('data-station-state="ahead"');
    expect(at("FH-1")).toContain('data-station-state="done"');
  });

  it("holds a blocked station instead of flagging a next stop", () => {
    expect(at("FH-3")).toContain('data-station-state="held"');
    expect(at("FH-3")).toContain(", blocked");
    expect(epic).not.toContain('data-tone="blocked"');
  });

  it("keeps the ordered line's track", () => {
    expect(markup).toContain("data-track");
    expect(markup).not.toContain('data-layout="any-order"');
  });
});
