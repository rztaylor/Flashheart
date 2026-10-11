import { describe, expect, it } from "vitest";

import type { Workstream, WorkstreamTicket } from "../../api/board";
import { isFinished, keptExpanded } from "./collapse";

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

const workstream = (
  slug: string,
  tickets: WorkstreamTicket[],
  extra: Partial<Workstream> = {},
): Workstream => ({
  slug,
  title: slug,
  status: "completed",
  suspended: false,
  declaredStatus: "",
  priority: "",
  created: "",
  done: 0,
  total: tickets.length,
  next: "",
  blockedBy: [],
  dependsOnWorkstreams: [],
  tags: [],
  needsRepair: [],
  warnings: [],
  tickets,
  ...extra,
});

// A workstream is finished, and collapses, only when every ticket is Done,
// Rejected or Archived (FH-68); Ready to review keeps it open.
describe("isFinished", () => {
  it("is true when every ticket is Done, Rejected or Archived", () => {
    expect(
      isFinished(
        workstream("lines", [
          ticket("FH-1", "done"),
          ticket("FH-2", "done", { rejected: true }),
          ticket("FH-3", "archived"),
        ]),
      ),
    ).toBe(true);
  });

  it("is false while any ticket waits in Ready to review or earlier", () => {
    for (const column of [
      "review",
      "in-progress",
      "up-next",
      "backlog",
    ] as const) {
      expect(
        isFinished(
          workstream("lines", [ticket("FH-1", "done"), ticket("FH-2", column)]),
        ),
      ).toBe(false);
    }
  });

  it("is false for a missing ticket, an empty line or one needing repair", () => {
    expect(
      isFinished(
        workstream("lines", [
          ticket("FH-1", "done"),
          ticket("FH-9", "", { missing: true }),
        ]),
      ),
    ).toBe(false);
    expect(isFinished(workstream("lines", []))).toBe(false);
    expect(
      isFinished(
        workstream("lines", [ticket("FH-1", "done")], {
          needsRepair: ["Lists FH-1 twice"],
        }),
      ),
    ).toBe(false);
  });
});

// The remembered choice lasts only while a workstream stays finished: the
// slugs of unfinished or deleted workstreams are dropped.
describe("keptExpanded", () => {
  const finished = workstream("done-line", [ticket("FH-1", "done")]);
  const reopened = workstream("reopened", [
    ticket("FH-2", "done"),
    ticket("FH-3", "review"),
  ]);

  it("keeps finished workstreams and drops unfinished or deleted ones", () => {
    expect(
      keptExpanded([finished, reopened], ["done-line", "reopened", "deleted"]),
    ).toEqual(["done-line"]);
  });

  it("returns the same list when nothing is dropped", () => {
    const slugs = ["done-line"];
    expect(keptExpanded([finished, reopened], slugs)).toBe(slugs);
  });
});
