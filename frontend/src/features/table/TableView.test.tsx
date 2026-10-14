import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Card } from "../../api/board";
import { noReason } from "../../api/runs";
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
          reason: noReason,
          lastActivity: "",
        },
      }),
    ]}
    lines={new Map()}
    workstreams={new Map()}
    onOpen={() => undefined}
  />,
);

// idLink is the ticket id as a link to its full page in a new tab (KEY-3).
const idLink = (id: string) =>
  new RegExp(
    `<a [^>]*href="#/ticket/${id}"[^>]*target="_blank"[^>]*>${id}</a>`,
  );
// insideButton finds a link nested in a button, which is not allowed.
const linkInButton = /<button[^>]*>(?:(?!<\/button>)[\s\S])*<a /;

describe("TableView", () => {
  it("links each id to the ticket's full page, beside the row's button", () => {
    expect(markup).toMatch(idLink("FH-1"));
    expect(markup).toMatch(idLink("FH-3"));
    expect(markup).not.toMatch(linkInButton);
  });

  it("heads the column state as Status, shown as pills", () => {
    expect(markup).toContain(">Status<");
    expect(markup).toContain('data-tone="progress"');
    expect(markup).toContain('data-tone="done"');
  });

  it("shows type and priority as tags", () => {
    expect(markup).toContain(">bug</span>");
    expect(markup).toContain(">High</span>");
  });

  it("marks the type and priority words with the card's icon and bars (FH-154)", () => {
    const cells = markup.split("<td");
    const type = cells.find((cell) => cell.includes("bug</span>")) ?? "";
    expect(type).toContain('data-type="bug"');
    expect(type).toContain('data-icon="bug"');
    expect(type.indexOf("data-type")).toBeLessThan(type.indexOf("bug</span>"));
    const priority = cells.find((cell) => cell.includes("High</span>")) ?? "";
    expect(priority).toContain('data-priority="high"');
    expect(priority.indexOf("data-priority")).toBeLessThan(
      priority.indexOf("High</span>"),
    );
  });

  it("shows a blocker as the blocked pill and a live run by its mark", () => {
    expect(markup).toContain('data-tone="blocked"');
    expect(markup).toContain('data-icon="blocked"');
    expect(markup).toContain('data-run-state="working"');
  });
});
