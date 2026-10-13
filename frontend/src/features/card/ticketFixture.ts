// A loaded ticket for the full page's interaction tests (FH-65, FH-66): a
// review with evidence and steps, notes, a handoff and three files. Test
// support only; nothing in the app imports it.
import type { Review, TicketDetail } from "../../api/board";
import type { Run } from "../../api/runs";

const reviewMarkdown = [
  "# Review: Outline the page",
  "",
  "## Summary",
  "",
  "Three columns.",
  "",
  "## Evidence",
  "",
  "- ![Light, 1440](files/light.png)",
  "- ![Dark, 1440](files/dark.png)",
  "",
  "## How to Verify",
  "",
  "1. Open the page",
  "2. Scroll it",
  "",
  "## Tests",
  "",
  "All pass.",
  "",
].join("\n");

export const review: Review = {
  markdown: reviewMarkdown,
  hash: "rh",
  before: reviewMarkdown.slice(0, reviewMarkdown.indexOf("## How to Verify")),
  intro: "\n",
  steps: [
    { text: "Open the page", done: true },
    { text: "Scroll it", done: false },
  ],
  outro: "\n",
  after: reviewMarkdown.slice(reviewMarkdown.indexOf("## Tests")),
};

export const body = [
  "# Outline the page",
  "",
  "## Description",
  "",
  "Lay the ticket out in three columns.",
  "",
  "## Acceptance Criteria",
  "",
  "- [x] Three columns",
  "- [ ] No tabs",
  "",
  "## Test Plan",
  "",
  "Red first.",
  "",
  "## Context",
  "",
  "## Notes",
  "",
  "- 2026-10-11 · claude:aaaa1111: Mocked the layout.",
  '- 2026-10-12 · Question from claude:aaaa1111 — "Drop the line strip?" Answer (Robert): "Yes, drop it".',
  "",
].join("\n");

const files = ["light.png", "dark.png", "extra.png"].map((file) => ({
  file,
  caption: `Caption of ${file}`,
  kind: "screenshot",
  run: "",
  added: "",
  url: `/api/files/${file}`,
}));

export function detail(overrides: Partial<TicketDetail> = {}): TicketDetail {
  return {
    project: "flashheart",
    id: "FH-65",
    slug: "outline",
    column: "review",
    title: "Outline the page",
    type: "feature",
    priority: "medium",
    workstream: "",
    tags: ["ui", "ticket-page"],
    created: "2026-10-11",
    updated: "",
    modified: "2026-10-12T10:00:00Z",
    branch: "feature/ticket-page-outline",
    pr: "",
    dependsOn: ["FH-99"],
    criteria: { done: 1, total: 2 },
    excerpt: "",
    handoffNext: "",
    attachments: 3,
    hasReview: true,
    blocked: false,
    blockedBy: [],
    needsRepair: [],
    warnings: [],
    needsYou: false,
    agentWorking: false,
    openQuestions: 0,
    body,
    frontmatter: "",
    session: "",
    gitRef: "",
    dependsOnWorkstreams: [],
    criteriaItems: [
      { text: "Three columns", done: true },
      { text: "No tabs", done: false },
    ],
    handoff: {
      markdown:
        "_Updated 2026-10-12 12:30 UTC by claude:aaaa1111 (run)._\n\n**Next**\n- Verify it",
      next: ["Verify it"],
    },
    review,
    attachmentFiles: files,
    hash: "abc",
    raw: "",
    runs: [] as Run[],
    questions: [],
    comments: [],
    history: [],
    activityHash: "",
    ...overrides,
  };
}
