import { describe, expect, it } from "vitest";

import type { Review } from "../api/board";
import {
  commentInFile,
  deleteNote,
  deleteRefusal,
  quoteChoices,
} from "./comments";
import type { PageInput } from "./ticketSections";

const reviewMarkdown = [
  "# Review: Outline the page",
  "",
  "## Summary",
  "",
  "Three columns.",
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

const review: Review = {
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

function detail(overrides: Partial<PageInput> = {}): PageInput {
  return {
    type: "feature",
    body: [
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
      "",
    ].join("\n"),
    needsRepair: [],
    questions: [],
    handoff: null,
    blockedBy: [],
    criteriaItems: [
      { text: "Three columns", done: true },
      { text: "No tabs", done: false },
    ],
    review,
    attachmentFiles: [],
    ...overrides,
  };
}

describe("quoteChoices", () => {
  it("offers the ticket's sections, each criterion and each How to verify step", () => {
    const groups = quoteChoices(detail());
    expect(groups.map((group) => group.label)).toEqual([
      "Sections",
      "Criteria",
      "How to verify steps",
    ]);
    expect(groups[0]?.choices.map((choice) => choice.from)).toEqual([
      "Description",
      "Test plan",
      "Review: Summary",
      "Review: Tests",
    ]);
    expect(groups[1]?.choices).toEqual([
      {
        key: "criterion-1",
        label: "1. Three columns",
        from: "Acceptance criteria 1",
        text: "Three columns",
      },
      {
        key: "criterion-2",
        label: "2. No tabs",
        from: "Acceptance criteria 2",
        text: "No tabs",
      },
    ]);
    expect(groups[2]?.choices.map((choice) => choice.from)).toEqual([
      "How to verify 1",
      "How to verify 2",
    ]);
    expect(groups[0]?.choices[0]).toMatchObject({
      text: "Lay the ticket out in three columns.",
    });
  });

  it("leaves out empty groups and cuts a long section to a quote's size", () => {
    const long = Array.from(
      { length: 80 },
      (_, n) => `Line ${n} of a long description.`,
    ).join("\n");
    const groups = quoteChoices(
      detail({
        body: `# T\n\n## Description\n\n${long}\n`,
        criteriaItems: [],
        review: null,
      }),
    );
    expect(groups.map((group) => group.label)).toEqual(["Sections"]);
    const text = groups[0]?.choices[0]?.text ?? "";
    expect(text.length).toBeLessThanOrEqual(1001);
    expect(text.endsWith("…")).toBe(true);
    expect(text.startsWith("Line 0 of a long description.\nLine 1")).toBe(true);
  });
});

describe("commentInFile", () => {
  const file = [
    "# Activity: FH-42",
    "",
    "### 2026-10-13T09:10:53Z · history · Robert · 0a1b2c3d",
    "",
    "- created: backlog",
    "",
    "### 2026-10-13T09:12:00Z · comment · Robert · 4e5f6a7b",
    "",
    "state: open",
    "",
    "They do not keep it in Done.",
    "",
    "#### Not an entry",
    "",
    "### 2026-10-14T08:02:11Z · history · claude:5b0c7e2a · 2a3b4c5d",
    "",
    "- moved: up-next → in-progress",
    "",
  ].join("\n");

  it("is one comment's entry in the activity file, as the conflict dialog shows it", () => {
    expect(commentInFile(file, "4e5f6a7b")).toBe(
      [
        "### 2026-10-13T09:12:00Z · comment · Robert · 4e5f6a7b",
        "",
        "state: open",
        "",
        "They do not keep it in Done.",
        "",
        "#### Not an entry",
      ].join("\n"),
    );
  });

  it("is the whole file when the comment is not in it", () => {
    expect(commentInFile(file, "ffffffff")).toBe(file);
  });
});

describe("deleting a comment (FH-140)", () => {
  const plain = { state: "open", delivery: "", sentTo: "" };

  it("allows a comment, one still waiting for its session and a resolved request", () => {
    expect(deleteRefusal(plain)).toBeUndefined();
    expect(
      deleteRefusal({
        ...plain,
        delivery: "waiting",
        sentTo: "claude:aaaa1111",
      }),
    ).toBeUndefined();
    expect(
      deleteRefusal({
        ...plain,
        delivery: "missed",
        sentTo: "claude:aaaa1111",
      }),
    ).toBeUndefined();
    expect(deleteRefusal({ ...plain, state: "resolved" })).toBeUndefined();
  });

  it("says why one delivered or an open or addressed request cannot go", () => {
    expect(
      deleteRefusal({
        ...plain,
        delivery: "delivered",
        sentTo: "claude:aaaa1111",
      }),
    ).toBe("It can't be deleted: claude:aaaa1111 has read it.");
    expect(deleteRefusal({ ...plain, state: "change-requested" })).toBe(
      "It can't be deleted while the change it asks for is open. Resolve it first.",
    );
    expect(deleteRefusal({ ...plain, state: "addressed" })).toBe(
      "It can't be deleted while the change request waits for you. Resolve it first.",
    );
  });

  it("says a waiting comment is withdrawn and history keeps no text", () => {
    expect(deleteNote(plain)).toBe(
      "The history keeps that you deleted a comment, not what it said.",
    );
    expect(
      deleteNote({ ...plain, delivery: "waiting", sentTo: "claude:aaaa1111" }),
    ).toBe(
      "claude:aaaa1111 has not taken it yet and never will. The history keeps that you deleted a comment, not what it said.",
    );
  });
});
