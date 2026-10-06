import { describe, expect, it } from "vitest";

import type { Archived } from "../api/archive";
import { confirmsDelete, filterArchived } from "./archive";

const item = (id: string, title: string, patch: Partial<Archived> = {}) =>
  ({
    id,
    title,
    type: "feature",
    priority: "medium",
    workstream: "",
    column: "backlog",
    archived: "2026-10-06T10:00:00Z",
    attachments: 0,
    hasReview: false,
    ...patch,
  }) as Archived;

describe("archive", () => {
  const items = [
    item("FH-7", "Old idea"),
    item("FH-12", "Offline mode", { type: "spike", workstream: "sync" }),
  ];

  it("finds archived tickets by id, title, type and workstream", () => {
    expect(filterArchived(items, "")).toEqual(items);
    expect(filterArchived(items, "fh-7").map((x) => x.id)).toEqual(["FH-7"]);
    expect(filterArchived(items, "offline").map((x) => x.id)).toEqual([
      "FH-12",
    ]);
    expect(filterArchived(items, "spike").map((x) => x.id)).toEqual(["FH-12"]);
    expect(filterArchived(items, "sync").map((x) => x.id)).toEqual(["FH-12"]);
    expect(filterArchived(items, "nothing")).toEqual([]);
  });

  it("deletes only when the exact id is typed", () => {
    expect(confirmsDelete("FH-7", "FH-7")).toBe(true);
    expect(confirmsDelete(" FH-7 ", "FH-7")).toBe(true);
    expect(confirmsDelete("fh-7", "FH-7")).toBe(false);
    expect(confirmsDelete("FH-70", "FH-7")).toBe(false);
    expect(confirmsDelete("", "FH-7")).toBe(false);
  });
});
