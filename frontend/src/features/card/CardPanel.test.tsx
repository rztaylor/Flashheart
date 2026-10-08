import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { TicketDetail } from "../../api/board";
import { TicketLinksInPlace } from "../../components/TicketLink";
import { PanelHeader, ReviewTab, TicketTab } from "./CardPanel";

// The card panel of docs/dev/specs/ui-layout.md §3.
function detail(overrides: Partial<TicketDetail> = {}): TicketDetail {
  return {
    project: "flashheart",
    id: "FH-4",
    slug: "mcp",
    column: "in-progress",
    title: "MCP server, claims, checkpoints and questions",
    type: "feature",
    priority: "high",
    workstream: "agent-protocol",
    tags: [],
    created: "2026-10-05",
    updated: "",
    modified: "2026-10-06T10:00:00Z",
    branch: "feature/mcp-protocol",
    dependsOn: [],
    criteria: { done: 1, total: 2 },
    excerpt: "",
    handoffNext: "",
    attachments: 0,
    hasReview: false,
    blocked: true,
    blockedBy: [
      {
        kind: "ticket",
        text: "Depends on FH-3, which is in Backlog",
        missing: false,
      },
    ],
    needsRepair: [],
    warnings: [],
    needsYou: true,
    agentWorking: false,
    openQuestions: 1,
    body: "# MCP server\n\n## Description\n\nServe the tools.",
    frontmatter: "",
    session: "",
    gitRef: "",
    dependsOnWorkstreams: [],
    criteriaItems: [
      { text: "Server starts", done: true },
      { text: "Claims hold", done: false },
    ],
    handoff: {
      markdown: "## Handoff\n\nNext: verify.",
      next: ["Verify the live session"],
    },
    review: null,
    attachmentFiles: [],
    hash: "abc",
    raw: "",
    runs: [],
    questions: [
      {
        id: "q1",
        run: "claude:1234abcd",
        ticket: "FH-4",
        kind: "decision",
        text: "Which session should we verify first?",
        options: ["Claude", "Codex"],
        asked: "2026-10-06T11:00:00Z",
      },
    ],
    ...overrides,
  };
}

// idLink is the ticket id as a link to its full page in a new tab (KEY-3).
const idLink = (id: string) =>
  new RegExp(
    `<a [^>]*href="#/ticket/${id}"[^>]*target="_blank"[^>]*>${id}</a>`,
  );

describe("PanelHeader", () => {
  const markup = renderToStaticMarkup(
    <PanelHeader detail={detail()} workstreamTitle="Agent protocol" />,
  );

  it("leads with the id, then the title as the panel heading", () => {
    const id = markup.indexOf(">FH-4<");
    const title = markup.indexOf("<h2");
    expect(id).toBeGreaterThanOrEqual(0);
    expect(id).toBeLessThan(title);
  });

  it("links the id to the ticket's full page", () => {
    expect(markup).toMatch(idLink("FH-4"));
  });

  it("states the column, blocker and priority as pills and tags", () => {
    expect(markup).toContain('data-tone="progress"');
    expect(markup).toContain(">In progress<");
    expect(markup).toContain('data-tone="blocked"');
    expect(markup).toContain(">High priority<");
  });
});

describe("TicketTab", () => {
  const markup = renderToStaticMarkup(
    <TicketTab
      detail={detail()}
      keys={new Set(["FH"])}
      onOpen={() => undefined}
      onToggle={() => Promise.resolve(true)}
      onAnswer={() => Promise.resolve(undefined)}
    />,
  );

  it("orders questions, handoff, blockers, then criteria", () => {
    const at = (text: string) => {
      const index = markup.indexOf(text);
      expect(index, text).toBeGreaterThanOrEqual(0);
      return index;
    };
    const question = at("Question for you");
    const handoff = at(">Handoff<");
    const blocked = at(">Blocked by<");
    const criteria = at(">Acceptance criteria<");
    expect(question).toBeLessThan(handoff);
    expect(handoff).toBeLessThan(blocked);
    expect(blocked).toBeLessThan(criteria);
  });

  it("sets the handoff and the question as callouts", () => {
    expect(markup).toContain("bg-callout-handoff");
    expect(markup).toContain("bg-callout-question");
  });

  it("links ticket ids in the text, criteria, handoff and blockers", () => {
    const linked = renderToStaticMarkup(
      <TicketTab
        detail={detail({
          body: "# T\n\n## Description\n\nFollows FH-2.",
          criteriaItems: [{ text: "Works like FH-5", done: false }],
          handoff: { markdown: "", next: ["Pair with FH-6"] },
        })}
        keys={new Set(["FH"])}
        onOpen={() => undefined}
      />,
    );
    for (const id of ["FH-2", "FH-5", "FH-6", "FH-3"])
      expect(linked, id).toMatch(idLink(id));
  });

  it("keeps ticket links in the same tab on the full page", () => {
    const inPlace = renderToStaticMarkup(
      <TicketLinksInPlace>
        <TicketTab
          detail={detail({ body: "# T\n\nFollows FH-2." })}
          keys={new Set(["FH"])}
          onOpen={() => undefined}
        />
      </TicketLinksInPlace>,
    );
    expect(inPlace).toContain('href="#/ticket/FH-2"');
    expect(inPlace).not.toContain('target="_blank"');
  });

  it("offers the agent's options as answers", () => {
    expect(markup).toContain(">Claude<");
    expect(markup).toContain(">Codex<");
  });
});

describe("ReviewTab", () => {
  const review = {
    markdown: "",
    hash: "def",
    before: "# Review\n\nThe summary.\n",
    intro: "Start the server.\n",
    steps: [
      { text: "Open the board.", done: true },
      { text: "Drag `FH-1` to Done.", done: false },
    ],
    outro: "Both themes.\n",
    after: "## Risks\n\nNone.\n",
  };
  const render = (onVerify?: () => Promise<boolean>) =>
    renderToStaticMarkup(
      <ReviewTab
        detail={detail({ review })}
        keys={new Set(["FH"])}
        onVerify={onVerify}
      />,
    );

  it("shows How to Verify steps as a checklist between their prose", () => {
    const markup = render(() => Promise.resolve(true));
    const at = (text: string) => {
      const index = markup.indexOf(text);
      expect(index, text).toBeGreaterThanOrEqual(0);
      return index;
    };
    expect(at("The summary.")).toBeLessThan(at(">How to Verify<"));
    expect(at(">How to Verify<")).toBeLessThan(at("Start the server."));
    expect(at("Start the server.")).toBeLessThan(at("Open the board."));
    expect(at("Open the board.")).toBeLessThan(at("Both themes."));
    expect(at("Both themes.")).toBeLessThan(at(">Risks<"));
    expect(markup).toContain("1 of 2");
    expect(markup).toContain("<code>FH-1</code>");
    expect(markup.match(/type="checkbox"/g)).toHaveLength(2);
  });

  it("shows the steps without boxes to tick when read-only", () => {
    const markup = render();
    expect(markup).not.toContain('type="checkbox"');
    expect(markup).toContain("Done: ");
  });

  it("renders a review without steps as markdown", () => {
    const markup = renderToStaticMarkup(
      <ReviewTab
        detail={detail({
          review: {
            ...review,
            markdown: "# Review\n\nJust prose.",
            steps: [],
          },
        })}
        keys={new Set(["FH"])}
      />,
    );
    expect(markup).toContain("Just prose.");
    expect(markup).not.toContain("How to Verify");
  });
});
