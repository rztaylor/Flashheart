import { describe, expect, it } from "vitest";

import { viewSummary } from "./summary";

// The page header's one quiet line (ui-layout.md §1).
const noCounts = {
  backlog: 0,
  "up-next": 0,
  "in-progress": 0,
  review: 0,
  done: 0,
};

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

  it("counts what is in progress and ready to review on the Overview", () => {
    expect(
      viewSummary({
        ...base,
        view: "overview",
        counts: [
          { ...noCounts, "in-progress": 2, review: 1 },
          { ...noCounts, "in-progress": 1 },
        ],
      }),
    ).toBe("3 in progress · 1 ready to review");
    expect(viewSummary({ ...base, view: "overview" })).toBe("");
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

  it("counts archived tickets or projects", () => {
    expect(
      viewSummary({
        ...base,
        view: "archive",
        archived: { count: 1, of: "tickets" },
      }),
    ).toBe("1 archived ticket");
    expect(
      viewSummary({
        ...base,
        view: "archive",
        archived: { count: 3, of: "projects" },
      }),
    ).toBe("3 archived projects");
    expect(viewSummary({ ...base, view: "archive" })).toBe("");
  });
});
