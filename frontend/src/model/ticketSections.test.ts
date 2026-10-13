import { describe, expect, it } from "vitest";

import type { Attachment, Comment, Review, TicketDetail } from "../api/board";
import type { Question, Run } from "../api/runs";
import {
  type PageInput,
  pageSections,
  sectionLabel,
  ticketSections,
  unreferencedFiles,
} from "./ticketSections";

const run = (id: string, parent = "") => ({ id, parent }) as Run;
const file = (name: string) => ({ file: name }) as Attachment;
const review = { markdown: "# Review", steps: [] } as unknown as Review;

describe("ticketSections", () => {
  it("offers Ticket and Runs for a plain read-only ticket", () => {
    const sections = ticketSections(
      { runs: [], attachmentFiles: [], review: null },
      false,
    );
    expect(sections).toEqual([
      { id: "ticket", title: "Ticket" },
      { id: "runs", title: "Runs" },
    ]);
  });

  it("orders Ticket, Edit, Runs, Attachments, Review with their counts", () => {
    const sections = ticketSections(
      {
        runs: [run("claude:a"), run("claude:a/x", "claude:a"), run("claude:b")],
        attachmentFiles: [file("a.png"), file("b.log")],
        review,
      },
      true,
    );
    expect(sections.map((section) => section.id)).toEqual([
      "ticket",
      "edit",
      "runs",
      "attachments",
      "review",
    ]);
    expect(sections.map(sectionLabel)).toEqual([
      "Ticket",
      "Edit",
      "Runs 2",
      "Attachments 2",
      "Review",
    ]);
  });

  it("leaves Review out without review.md", () => {
    const sections = ticketSections(
      {
        runs: [run("claude:a")],
        attachmentFiles: [file("a.png")],
        review: null,
      },
      true,
    );
    expect(sections.map(sectionLabel)).toEqual([
      "Ticket",
      "Edit",
      "Runs 1",
      "Attachments 1",
    ]);
  });
});

const body = [
  "# Outline the page",
  "",
  "## Description",
  "",
  "Lay it out.",
  "",
  "## Acceptance Criteria",
  "",
  "- [x] One",
  "- [ ] Two",
  "",
  "## Test Plan",
  "",
  "Red first. See ![sketch](files/sketch.png).",
  "",
  "## Context",
  "",
  "## Rollout",
  "",
  "Behind nothing.",
  "",
  "## Notes",
  "",
  "- 2026-10-11 · claude:2ff6eb1c: a note",
  "",
  "## Handoff",
  "",
  "**Next**",
  "- Verify",
  "",
].join("\n");

const reviewMarkdown = [
  "# Review: Outline the page",
  "",
  "**Work Item:** FH-65",
  "",
  "## Summary",
  "",
  "Three columns.",
  "",
  "## Key Files",
  "",
  "| File | What |",
  "",
  "## Evidence",
  "",
  "- ![Light, 1440](files/light.png)",
  "- ![Dark, 1440](files/dark.png)",
  "",
  "## How to Verify",
  "",
  "### Steps",
  "1. Open it",
  "2. [x] Scroll",
  "",
  "## Risks / Things to Watch",
  "",
  "- None",
  "",
].join("\n");

const pageReview: Review = {
  markdown: reviewMarkdown,
  hash: "h",
  before: reviewMarkdown.slice(0, reviewMarkdown.indexOf("## How to Verify")),
  intro: "\n",
  steps: [
    { text: "Open it", done: false },
    { text: "Scroll", done: true },
  ],
  outro: "\n",
  after: reviewMarkdown.slice(reviewMarkdown.indexOf("## Risks")),
};

const attachment = (name: string, kind = "screenshot") =>
  ({ file: name, caption: name, kind, url: `/f/${name}` }) as Attachment;

function input(overrides: Partial<PageInput> = {}): PageInput {
  return {
    type: "feature",
    body,
    needsRepair: [],
    questions: [],
    handoff: null,
    blockedBy: [],
    criteriaItems: [
      { text: "One", done: true },
      { text: "Two", done: false },
    ],
    review: null,
    attachmentFiles: [],
    ...overrides,
  };
}

const ids = (detail: PageInput, activity = 0) =>
  pageSections(detail, activity).map((section) => section.id);

describe("pageSections", () => {
  it("orders the ticket's own sections, then Activity, without a review", () => {
    expect(ids(input())).toEqual([
      "criteria",
      "description",
      "test-plan",
      "context",
      "rollout",
      "activity",
    ]);
  });

  it("leads with repair, questions, handoff and blockers when present", () => {
    const detail = input({
      needsRepair: ["Bad frontmatter"],
      questions: [{ id: "q1" } as Question],
      handoff: { markdown: "", next: ["Verify"] },
      blockedBy: [{ kind: "ticket", text: "Depends on FH-1", missing: false }],
    });
    expect(ids(detail).slice(0, 5)).toEqual([
      "repair",
      "questions",
      "handoff",
      "blocked",
      "criteria",
    ]);
  });

  it("heads the page with change requests, counting those resolved (REV-6)", () => {
    const request = (id: string, state: string) =>
      ({ id, state, byPerson: true }) as Comment;
    const detail = input({
      needsRepair: ["Bad frontmatter"],
      questions: [{ id: "q1" } as Question],
      comments: [
        request("a", "change-requested"),
        request("b", "resolved"),
        request("c", "open"),
      ],
    });
    expect(ids(detail).slice(0, 3)).toEqual([
      "repair",
      "requests",
      "questions",
    ]);
    const section = pageSections(detail, 0).find((s) => s.id === "requests");
    expect(section).toMatchObject({
      title: "Change requests",
      count: "1/2",
      complete: false,
    });
    expect(
      ids(input({ comments: [request("c", "open")] })).includes("requests"),
    ).toBe(false);
  });

  it("follows the ticket with the review in its own sections, then Files and Activity", () => {
    const detail = input({
      review: pageReview,
      attachmentFiles: [
        attachment("light.png"),
        attachment("dark.png"),
        attachment("run.log", "log"),
      ],
    });
    expect(ids(detail, 3)).toEqual([
      "criteria",
      "description",
      "test-plan",
      "context",
      "rollout",
      "review",
      "review-summary",
      "review-key-files",
      "review-evidence",
      "review-how-to-verify",
      "review-risks-things-to-watch",
      "files",
      "activity",
    ]);
  });

  it("counts criteria, evidence, verify steps, files and activity", () => {
    const sections = pageSections(
      input({
        review: pageReview,
        attachmentFiles: [attachment("light.png"), attachment("run.log")],
      }),
      4,
    );
    const count = (id: string) =>
      sections.find((section) => section.id === id)?.count;
    expect(count("criteria")).toBe("1/2");
    expect(count("review-evidence")).toBe("2");
    expect(count("review-how-to-verify")).toBe("1/2");
    expect(count("files")).toBe("1");
    expect(count("activity")).toBe("4");
    expect(count("description")).toBeUndefined();
  });

  it("marks empty template sections and names them in sentence case", () => {
    const sections = pageSections(input({ criteriaItems: [] }), 0);
    const byId = new Map(sections.map((section) => [section.id, section]));
    expect(byId.get("context")).toMatchObject({
      title: "Context",
      empty: true,
    });
    expect(byId.get("criteria")).toMatchObject({
      title: "Acceptance criteria",
      empty: true,
    });
    expect(byId.get("test-plan")).toMatchObject({
      title: "Test plan",
      empty: false,
    });
    expect(byId.get("activity")?.empty).toBeFalsy();
  });

  it("lists a missing Reproduction as empty for a bug", () => {
    const sections = pageSections(
      input({ type: "bug", body: "# Bug\n\n## Description\n\nIt breaks." }),
      0,
    );
    expect(sections.map((section) => section.id)).toContain("reproduction");
    expect(
      sections.find((section) => section.id === "reproduction")?.empty,
    ).toBe(true);
  });

  it("gives the Evidence section its images and the rest of its text", () => {
    const evidence = pageSections(input({ review: pageReview }), 0).find(
      (section) => section.kind === "evidence",
    );
    expect(evidence?.images).toEqual([
      { alt: "Light, 1440", src: "files/light.png" },
      { alt: "Dark, 1440", src: "files/dark.png" },
    ]);
    expect(evidence?.markdown?.trim()).toBe("");
  });

  it("splits a review without steps into its sections", () => {
    const review = { ...pageReview, steps: [], before: reviewMarkdown };
    const sections = pageSections(input({ review }), 0);
    const verify = sections.find(
      (section) => section.id === "review-how-to-verify",
    );
    expect(verify?.kind).toBe("review-text");
    expect(verify?.count).toBeUndefined();
  });

  it("names the file's section each editable one saves to (FH-66)", () => {
    const sections = pageSections(
      input({
        review: pageReview,
        handoff: { markdown: "", next: ["Verify"] },
        body: "# T\n\n## description\n\nA\n\n## Rollout\n\nB\n\n## Rollout\n\nC",
      }),
      1,
    );
    const heading = new Map(
      sections.map((section) => [section.id, section.heading]),
    );
    // As written in the file, else the template's name for a missing one.
    expect(heading.get("description")).toBe("description");
    expect(heading.get("criteria")).toBe("Acceptance Criteria");
    expect(heading.get("test-plan")).toBe("Test Plan");
    expect(heading.get("context")).toBe("Context");
    expect(heading.get("rollout")).toBe("Rollout");
    // A repeated heading saves only to its first section.
    expect(heading.get("rollout-2")).toBeUndefined();
    // Handoff, the review and Activity are not edited in place.
    for (const id of ["handoff", "review", "review-summary", "activity"])
      expect(heading.get(id)).toBeUndefined();
  });

  it("keeps section ids unique", () => {
    const sections = pageSections(
      input({
        body: "# T\n\n## Description\n\nA\n\n## Files\n\nB\n\n## Files\n\nC",
      }),
      0,
    );
    const list = sections.map((section) => section.id);
    expect(new Set(list).size).toBe(list.length);
  });
});

describe("unreferencedFiles", () => {
  it("leaves out files the review or the ticket links, so each shows once", () => {
    const files = unreferencedFiles(
      input({
        review: pageReview,
        attachmentFiles: [
          attachment("light.png"),
          attachment("dark.png"),
          attachment("sketch.png"),
          attachment("other.png"),
          attachment("run.log", "log"),
        ],
      }) as Pick<TicketDetail, "body" | "review" | "attachmentFiles">,
    );
    expect(files.map((file) => file.file)).toEqual(["other.png", "run.log"]);
  });

  it("lists every file when nothing links them", () => {
    const files = unreferencedFiles({
      body: "# T",
      review: null,
      attachmentFiles: [attachment("a.png")],
    });
    expect(files).toHaveLength(1);
  });
});
