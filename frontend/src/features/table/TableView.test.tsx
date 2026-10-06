import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Card } from "../../api/board";
import { TableView } from "./TableView";

// The Table of docs/dev/specs/ui-layout.md §6: status as pills, type and
// priority as tags, blockers as the blocked pill, live runs as their mark.
function card(overrides: Partial<Card>): Card {
  return {
    project: "flashheart",
    id: "FH-1",
    slug: "x",
    column: "in-progress",
    title: "One",
    type: "bug",
    priority: "high",
    workstream: "",
    tags: [],
    created: "",
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

const markup = renderToStaticMarkup(
  <TableView
    cards={[
      card({}),
      card({
        id: "FH-2",
        column: "done",
        blocked: true,
        blockedBy: [
          { kind: "ticket", text: "Depends on FH-1", missing: false },
        ],
      }),
      card({
        id: "FH-3",
        column: "backlog",
        live: {
          run: "claude:abcd1234",
          short: "abcd",
          agent: "claude",
          state: "working",
          done: 1,
          total: 3,
          step: "Write tests",
          permission: "",
          waitingOn: "",
          question: "",
          questionAnswered: false,
          lastActivity: "",
        },
      }),
    ]}
    lines={new Map()}
    workstreams={new Map()}
    onOpen={() => undefined}
  />,
);

describe("TableView", () => {
  it("heads the column state as Status, shown as pills", () => {
    expect(markup).toContain(">Status<");
    expect(markup).toContain('data-tone="progress"');
    expect(markup).toContain('data-tone="done"');
  });

  it("shows type and priority as tags", () => {
    expect(markup).toContain(">bug</span>");
    expect(markup).toContain(">High</span>");
  });

  it("shows a blocker as the blocked pill and a live run by its mark", () => {
    expect(markup).toContain('data-tone="blocked"');
    expect(markup).toContain('data-run-state="working"');
  });
});
