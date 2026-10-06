import { describe, expect, it } from "vitest";

import { noRuns } from "../api/runs";
import { viewSummary } from "./summary";

// The page header's one quiet line (ui-layout.md §1).
describe("viewSummary", () => {
  const base = { filtered: false, workstreams: [] };

  it("counts tickets on the Board and Table, filtered or not", () => {
    expect(
      viewSummary({ ...base, view: "board", board: { shown: 3, total: 3 } }),
    ).toBe("3 tickets");
    expect(
      viewSummary({ ...base, view: "table", board: { shown: 1, total: 1 } }),
    ).toBe("1 ticket");
    expect(
      viewSummary({
        ...base,
        view: "board",
        filtered: true,
        board: { shown: 2, total: 9 },
      }),
    ).toBe("2 of 9 tickets");
    expect(viewSummary({ ...base, view: "board" })).toBe("");
  });

  it("counts live runs and who needs you on Agents", () => {
    expect(
      viewSummary({ ...base, view: "agents", runs: { ...noRuns, live: 1 } }),
    ).toBe("1 live run");
    expect(
      viewSummary({
        ...base,
        view: "agents",
        runs: { ...noRuns, live: 4, needsYou: 1 },
      }),
    ).toBe("4 live runs · 1 needs you");
    expect(
      viewSummary({
        ...base,
        view: "agents",
        runs: { ...noRuns, live: 3, needsYou: 2 },
      }),
    ).toBe("3 live runs · 2 need you");
    expect(viewSummary({ ...base, view: "agents" })).toBe("");
  });

  it("counts workstreams and stations served", () => {
    expect(
      viewSummary({
        ...base,
        view: "workstreams",
        workstreams: [
          { done: 2, total: 5 },
          { done: 1, total: 1 },
        ],
      }),
    ).toBe("2 workstreams · 3 of 6 stations served");
    expect(
      viewSummary({
        ...base,
        view: "workstreams",
        workstreams: [{ done: 0, total: 0 }],
      }),
    ).toBe("1 workstream · 0 of 0 stations served");
  });
});
