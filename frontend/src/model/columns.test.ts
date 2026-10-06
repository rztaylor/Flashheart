import { describe, expect, it } from "vitest";

import type { Card } from "../api/board";
import { shownVirtual, toggleVirtual, VIRTUAL_COLUMNS } from "./columns";

const card = (patch: Partial<Card>) =>
  ({ needsYou: false, agentWorking: false, ...patch }) as Card;

describe("virtual columns", () => {
  it("are Needs you then Agent working", () => {
    expect(VIRTUAL_COLUMNS.map((column) => column.id)).toEqual([
      "needs-you",
      "agent-working",
    ]);
  });

  it("show only when chosen and holding tickets", () => {
    const cards = [card({ needsYou: true }), card({})];
    expect(
      shownVirtual(cards, ["needs-you", "agent-working"]).map((c) => c.id),
    ).toEqual(["needs-you"]);
    expect(shownVirtual(cards, ["agent-working"])).toEqual([]);
    expect(
      shownVirtual([card({ agentWorking: true })], ["agent-working"]).map(
        (c) => c.id,
      ),
    ).toEqual(["agent-working"]);
  });

  it("toggle in their fixed order", () => {
    expect(toggleVirtual(["agent-working"], "needs-you", true)).toEqual([
      "needs-you",
      "agent-working",
    ]);
    expect(
      toggleVirtual(["needs-you", "agent-working"], "needs-you", false),
    ).toEqual(["agent-working"]);
    expect(toggleVirtual([], "needs-you", false)).toEqual([]);
  });
});
