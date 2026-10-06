import { describe, expect, it } from "vitest";

import { priorityLabel, statusOf } from "./status";

describe("statusOf", () => {
  it("names every column with its state tone", () => {
    expect(statusOf("backlog")).toEqual({ tone: "neutral", label: "Backlog" });
    expect(statusOf("up-next")).toEqual({ tone: "neutral", label: "Up next" });
    expect(statusOf("in-progress")).toEqual({
      tone: "progress",
      label: "In progress",
    });
    expect(statusOf("review")).toEqual({
      tone: "review",
      label: "Ready to review",
    });
    expect(statusOf("done")).toEqual({ tone: "done", label: "Done" });
  });

  it("treats archived and missing stations as neutral words", () => {
    expect(statusOf("archived")).toEqual({
      tone: "neutral",
      label: "Archived",
    });
    expect(statusOf("")).toEqual({ tone: "neutral", label: "Missing" });
  });
});

describe("priorityLabel", () => {
  it("names known priorities and keeps a file's own word", () => {
    expect(priorityLabel("high")).toBe("High");
    expect(priorityLabel("medium")).toBe("Medium");
    expect(priorityLabel("low")).toBe("Low");
    expect(priorityLabel("urgent")).toBe("urgent");
    expect(priorityLabel("")).toBe("");
  });
});
