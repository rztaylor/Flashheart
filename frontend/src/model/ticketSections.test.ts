import { describe, expect, it } from "vitest";

import type { Attachment, Review } from "../api/board";
import type { Run } from "../api/runs";
import { sectionLabel, ticketSections } from "./ticketSections";

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
