import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Card } from "../../api/board";
import { noReason } from "../../api/runs";
import { TicketCard } from "./TicketCard";

// A card's markers lie under its one covering button, so a click or a drag
// anywhere on the card reaches the button; the button takes the words of
// the marker under the pointer as its tooltip (FH-147, ui-layout.md §2).
const card = {
  project: "flashheart",
  id: "FH-7",
  slug: "x",
  column: "up-next",
  title: "Locked atomic ticket writes",
  type: "feature",
  priority: "high",
  workstream: "",
  tags: [],
  created: "2026-10-01",
  updated: "",
  modified: "2026-10-06T10:00:00Z",
  branch: "",
  pr: "",
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
  reason: noReason,
} as Card;

function setup() {
  const onOpen = vi.fn();
  const { container } = render(
    <TicketCard
      card={card}
      density="normal"
      tabIndex={0}
      now={new Date("2026-10-06T12:00:00Z")}
      onOpen={onOpen}
    />,
  );
  const button = screen.getByRole("button", {
    name: /^Locked atomic ticket writes, FH-7/,
  });
  const mark = (name: string) =>
    container.querySelector(`[data-mark="${name}"]`) as HTMLElement;
  return { button, mark, onOpen };
}

describe("TicketCard markers and the covering button", () => {
  const original = document.elementsFromPoint;
  afterEach(() => {
    document.elementsFromPoint = original;
  });

  it("keeps every marker under the button, so a click anywhere opens it", () => {
    const { mark } = setup();
    for (const name of ["type", "age", "priority"]) {
      expect(mark(name), name).not.toBeNull();
      expect(mark(name).className, name).not.toMatch(/\bz-\[/);
    }
  });

  it("shows the words of the marker under the pointer as its tooltip", () => {
    const { button, mark } = setup();
    document.elementsFromPoint = () => [button, mark("priority")];
    fireEvent.pointerMove(button, { clientX: 10, clientY: 10 });
    expect(button.getAttribute("title")).toBe("High priority");

    document.elementsFromPoint = () => [button, mark("type")];
    fireEvent.pointerMove(button, { clientX: 20, clientY: 10 });
    expect(button.getAttribute("title")).toBe("feature");

    document.elementsFromPoint = () => [button];
    fireEvent.pointerMove(button, { clientX: 30, clientY: 10 });
    expect(button.hasAttribute("title")).toBe(false);
  });

  it("drops the tooltip when the pointer leaves", () => {
    const { button, mark } = setup();
    document.elementsFromPoint = () => [button, mark("priority")];
    fireEvent.pointerMove(button, { clientX: 10, clientY: 10 });
    fireEvent.pointerLeave(button);
    expect(button.hasAttribute("title")).toBe(false);
  });
});
