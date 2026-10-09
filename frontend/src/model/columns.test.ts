import { describe, expect, it } from "vitest";

import { neighbourColumn, setColumnShown, shownColumns } from "./columns";

describe("hidden columns (FH-41)", () => {
  it("show every real column, Needs you never among them (FH-44), unless the Backlog is hidden", () => {
    expect(shownColumns([]).map((column) => column.id)).toEqual([
      "backlog",
      "up-next",
      "in-progress",
      "review",
      "done",
    ]);
    expect(shownColumns(["backlog"]).map((column) => column.id)).toEqual([
      "up-next",
      "in-progress",
      "review",
      "done",
    ]);
  });

  it("toggle the Backlog on and off", () => {
    expect(setColumnShown([], "backlog", false)).toEqual(["backlog"]);
    expect(setColumnShown(["backlog"], "backlog", true)).toEqual([]);
    expect(setColumnShown(["backlog"], "backlog", false)).toEqual(["backlog"]);
  });

  it("step to the next shown column, never a hidden Backlog", () => {
    expect(neighbourColumn([], "up-next", -1)).toBe("backlog");
    expect(neighbourColumn(["backlog"], "up-next", -1)).toBeUndefined();
    expect(neighbourColumn(["backlog"], "up-next", 1)).toBe("in-progress");
    expect(neighbourColumn([], "done", 1)).toBeUndefined();
  });
});
