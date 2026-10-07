import { describe, expect, it } from "vitest";

import type { Card } from "../api/board";
import {
  placeVirtual,
  shownVirtual,
  toggleVirtual,
  VIRTUAL_COLUMNS,
} from "./columns";

const card = (patch: Partial<Card>) =>
  ({ needsYou: false, agentWorking: false, ...patch }) as Card;

describe("virtual columns", () => {
  it("are Needs you alone; Agent working is a State filter (FH-42)", () => {
    expect(VIRTUAL_COLUMNS.map((column) => column.id)).toEqual(["needs-you"]);
  });

  it("show only when chosen and holding tickets", () => {
    const cards = [card({ needsYou: true }), card({})];
    expect(shownVirtual(cards, ["needs-you"]).map((c) => c.id)).toEqual([
      "needs-you",
    ]);
    expect(shownVirtual(cards, [])).toEqual([]);
    expect(shownVirtual([card({ agentWorking: true })], ["needs-you"])).toEqual(
      [],
    );
  });

  it("toggle on and off", () => {
    expect(toggleVirtual([], "needs-you", true)).toEqual(["needs-you"]);
    expect(toggleVirtual(["needs-you"], "needs-you", false)).toEqual([]);
    expect(toggleVirtual([], "needs-you", false)).toEqual([]);
  });

  it("stand between In progress and Ready to review", () => {
    const real = ["backlog", "up-next", "in-progress", "review", "done"].map(
      (id) => ({ id }),
    );
    const mirrors = [{ id: "needs-you" }];
    expect(placeVirtual(real, mirrors).map((column) => column.id)).toEqual([
      "backlog",
      "up-next",
      "in-progress",
      "needs-you",
      "review",
      "done",
    ]);
    expect(placeVirtual(real, [])).toEqual(real);
  });
});
