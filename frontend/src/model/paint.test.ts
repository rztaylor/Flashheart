import { describe, expect, it } from "vitest";

import type { Card } from "../api/board";
import { paintFor, paintKey } from "./paint";

function card(overrides: Partial<Card>): Card {
  return {
    project: "alpha",
    id: "AL-1",
    slug: "x",
    column: "backlog",
    title: "X",
    type: "feature",
    priority: "medium",
    workstream: "",
    tags: [],
    created: "2026-10-01",
    updated: "",
    modified: "",
    branch: "",
    dependsOn: [],
    criteria: { done: 0, total: 0 },
    excerpt: "",
    handoffNext: "",
    attachments: 0,
    hasReview: false,
    blocked: false,
    blockedBy: [],
    needsRepair: [],
    warnings: [],
    needsYou: false,
    agentWorking: false,
    openQuestions: 0,
    ...overrides,
  };
}

const now = new Date("2026-10-05T12:00:00Z");

describe("paintFor", () => {
  it("colours by type, with unknown types as other", () => {
    expect(paintFor(card({ type: "bug" }), "type", now)).toEqual({
      token: "type-bug",
      label: "bug",
    });
    expect(paintFor(card({ type: "chore" }), "type", now)).toEqual({
      token: "type-other",
      label: "chore",
    });
    expect(paintFor(card({ type: "" }), "type", now)).toBeUndefined();
  });

  it("colours by priority", () => {
    expect(paintFor(card({ priority: "high" }), "priority", now)).toEqual({
      token: "priority-high",
      label: "High",
    });
    expect(paintFor(card({ priority: "" }), "priority", now)).toBeUndefined();
  });

  it("colours by age since the ticket last changed", () => {
    const at = (iso: string) => paintFor(card({ modified: iso }), "age", now);
    expect(at("2026-10-05T02:00:00Z")?.label).toBe("Today");
    expect(at("2026-10-01T12:00:00Z")?.label).toBe("This week");
    expect(at("2026-09-20T12:00:00Z")?.label).toBe("Older");
    expect(at("")).toBeUndefined();
  });

  it("paints nothing when colouring is off", () => {
    expect(paintFor(card({ type: "bug" }), "none", now)).toBeUndefined();
  });
});

describe("paintKey", () => {
  it("lists the values present, in a fixed order", () => {
    const cards = [
      card({ type: "spike" }),
      card({ type: "bug" }),
      card({ type: "bug" }),
      card({ type: "feature" }),
    ];
    expect(paintKey(cards, "type", now).map((entry) => entry.label)).toEqual([
      "feature",
      "bug",
      "spike",
    ]);
    expect(
      paintKey(
        [card({ priority: "low" }), card({ priority: "high" })],
        "priority",
        now,
      ).map((entry) => entry.label),
    ).toEqual(["High", "Low"]);
    expect(paintKey(cards, "none", now)).toEqual([]);
  });
});
