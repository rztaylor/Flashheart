import { isValidElement, type ReactElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import type { Card } from "../../api/board";
import type { Run } from "../../api/runs";
import { QuestionCard } from "../../components/QuestionCard";
import { overview } from "../../model/overview";
import { DecisionRow, OverviewSections, UnticketedRow } from "./OverviewView";

const now = new Date("2026-10-05T12:00:00Z");
const names = new Map([
  ["alpha", "Alpha"],
  ["beta", "Beta"],
]);

function card(patch: Partial<Card>): Card {
  return {
    project: "alpha",
    id: "AL-1",
    slug: "x",
    column: "in-progress",
    title: "Card panel",
    type: "feature",
    priority: "medium",
    workstream: "",
    tags: [],
    created: "2026-10-01",
    updated: "",
    modified: "2026-10-05T09:00:00Z",
    branch: "",
    dependsOn: [],
    criteria: { done: 2, total: 5 },
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
    ...patch,
  };
}

function run(id: string, patch: Partial<Run> = {}): Run {
  return {
    id,
    short: id.slice(0, 15),
    agent: "claude",
    kind: "session",
    parent: "",
    children: [],
    project: "alpha",
    cwd: "",
    branch: "main",
    worktree: "",
    source: "",
    agentType: "",
    state: "working",
    ticket: "",
    ticketTitle: "",
    linkedBy: "",
    dirty: false,
    noHandoff: false,
    permission: "",
    started: "2026-10-05T10:00:00Z",
    lastActivity: "2026-10-05T11:50:00Z",
    ended: "",
    endReason: "",
    tools: 0,
    edits: 0,
    files: [],
    plan: [],
    progress: { done: 0, total: 0 },
    questions: [],
    ...patch,
  };
}

const asking = run("claude:3f2a9c1e-0000", {
  state: "needs-you",
  ticket: "AL-1",
  questions: [
    {
      id: "q-1",
      run: "claude:3f2a9c1e-0000",
      ticket: "AL-1",
      kind: "decision",
      text: "Keep the old schema?",
      options: ["Keep it", "New one"],
      asked: "2026-10-05T11:56:00Z",
    },
  ],
});
const permission = run("claude:b7c4e9f2-0000", {
  state: "needs-you",
  permission: "Bash",
  project: "beta",
  branch: "spike/offline",
  lastActivity: "2026-10-05T11:48:00Z",
});

const noop = () => undefined;
const sections = (cards: Card[], runs: Run[], answer = true) =>
  renderToStaticMarkup(
    <OverviewSections
      data={overview(cards, runs)}
      now={now}
      projectNames={names}
      onOpen={noop}
      onReview={noop}
      onAnswer={answer ? async () => undefined : undefined}
      onCreateTicket={noop}
    />,
  );

// find walks an element tree, through every prop that holds elements, for
// the first element that matches; it does not render components.
function find(
  node: ReactNode,
  match: (element: ReactElement<Record<string, unknown>>) => boolean,
): ReactElement<Record<string, unknown>> | undefined {
  if (Array.isArray(node)) {
    for (const child of node) {
      const found = find(child, match);
      if (found) return found;
    }
    return undefined;
  }
  if (!isValidElement<Record<string, unknown>>(node)) return undefined;
  if (match(node)) return node;
  for (const value of Object.values(node.props)) {
    const found = find(value as ReactNode, match);
    if (found) return found;
  }
  return undefined;
}

describe("OverviewSections", () => {
  it("shows every section in order, each with its calm empty sentence", () => {
    const markup = sections([], []);
    const order = [
      "Needs your decision",
      "Ready for your review",
      "At risk",
      "In progress",
      "Work with no ticket",
      "Up next",
    ].map((title) => markup.indexOf(title));
    expect(order.every((at) => at >= 0)).toBe(true);
    expect([...order].sort((a, b) => a - b)).toEqual(order);
    for (const sentence of [
      "Nothing needs your decision.",
      "Nothing is waiting for your review.",
      "Nothing is at risk.",
      "Nothing is in progress.",
      "Every live session has a ticket.",
      "Up next is empty.",
    ])
      expect(markup).toContain(sentence);
    // Nothing to open: no chevrons and no Review results.
    expect(markup).not.toContain("aria-expanded");
  });

  it("answers a question in place and says where a permission is answered, with no button", () => {
    const markup = sections([card({})], [asking, permission]);
    expect(markup).toContain("Keep the old schema?");
    expect(markup).toContain(">Keep it</button>");
    expect(markup).toContain(">Send answer</button>");
    expect(markup).toContain("Permission for Bash");
    expect(markup).toContain("Answer in the session · Beta · spike/offline");
    // A read-only board shows the question without an answer form.
    expect(sections([card({})], [asking], false)).not.toContain("Send answer");
  });

  it("offers Review results, says subagents and at-risk reasons in words, and keeps tiles closed", () => {
    const markup = sections(
      [
        card({
          id: "AL-2",
          title: "Needs you filter",
          column: "review",
          criteria: { done: 4, total: 4 },
          updated: "2026-10-04T12:00:00Z",
          sessions: {
            run: "claude:s",
            short: "claude:s",
            agent: "claude",
            state: "ended",
            lastActivity: "2026-10-04T12:00:00Z",
            noLiveSession: false,
            dirty: false,
            noHandoff: false,
            subagents: { done: 0, running: 0, needsYou: 0 },
          },
        }),
        card({
          id: "AL-3",
          sessions: {
            run: "claude:w",
            short: "claude:w",
            agent: "claude",
            state: "working",
            lastActivity: "2026-10-05T11:50:00Z",
            noLiveSession: false,
            dirty: false,
            noHandoff: false,
            subagents: { done: 3, running: 2, needsYou: 0 },
          },
        }),
        card({
          id: "AL-4",
          title: "MCP-started runs",
          sessions: {
            run: "claude:e",
            short: "claude:e",
            agent: "claude",
            state: "ended",
            lastActivity: "2026-10-04T11:00:00Z",
            noLiveSession: true,
            dirty: false,
            noHandoff: false,
            subagents: { done: 0, running: 0, needsYou: 0 },
          },
        }),
      ],
      [run("claude:free")],
    );
    expect(markup).toContain("4/4 criteria met");
    expect(markup).toContain("Waiting 1 day");
    expect(markup).toMatch(/Review results/);
    expect(markup).toContain("Working · subagents 3 done · 2 running");
    expect(markup).toContain("No live session");
    expect(markup).toContain("Last activity 1 day ago");
    // At risk names no agent for a ticket no session works on.
    const risk = markup.slice(
      markup.indexOf("At risk"),
      markup.indexOf("In progress"),
    );
    expect(risk).not.toContain("Claude");
    // Work with no ticket is a closed tile with its count.
    expect(markup).toMatch(/aria-expanded="false"[^>]*>.*Work with no ticket/);
    expect(markup).not.toContain("Create ticket");
  });
});

describe("DecisionRow", () => {
  it("sends an answer for its question", async () => {
    const onAnswer = vi.fn(async () => undefined);
    const [item] = overview([card({})], [asking]).decision;
    if (!item) throw new Error("no decision");
    const tree = DecisionRow({
      item,
      now,
      projectNames: names,
      onOpen: noop,
      onAnswer,
    });
    const form = find(tree, (element) => element.type === QuestionCard);
    const send = form?.props.onAnswer as (answer: string) => Promise<unknown>;
    await send("Keep it");
    expect(onAnswer).toHaveBeenCalledWith("q-1", "Keep it");
  });
});

describe("UnticketedRow", () => {
  it("creates a ticket in the session's project", () => {
    const onCreateTicket = vi.fn();
    const session = run("claude:e2d8f6a4-0000", {
      project: "beta",
      branch: "spike/offline-sync",
      state: "waiting",
    });
    const tree = UnticketedRow({
      run: session,
      now,
      projectNames: names,
      onCreateTicket,
    });
    const markup = renderToStaticMarkup(tree);
    expect(markup).toContain("Beta · spike/offline-sync");
    expect(markup).toContain("Waiting");
    const button = find(
      tree,
      (element) =>
        element.type !== "span" &&
        typeof element.props.onClick === "function" &&
        renderToStaticMarkup(element).includes("Create ticket"),
    );
    if (!button) throw new Error("no Create ticket button");
    (button.props.onClick as () => void)();
    expect(onCreateTicket).toHaveBeenCalledWith("beta");
  });
});
