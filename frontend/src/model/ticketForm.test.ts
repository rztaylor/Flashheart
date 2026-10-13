import { describe, expect, it } from "vitest";

import {
  changes,
  fieldsText,
  formOf,
  split,
  type TicketForm,
  type TicketFormSource,
} from "./ticketForm";

const ticket: TicketFormSource = {
  title: "Split API handlers",
  column: "in-progress",
  type: "refactor",
  priority: "medium",
  created: "2026-10-05",
  branch: "feature/split-api",
  pr: "",
  workstream: "api",
  dependsOn: ["FH-12", "NG-3"],
  tags: ["tech-debt", "api"],
};

// The Edit tab's typed fields (EDIT-6).
describe("formOf", () => {
  it("reads a ticket's fields as the form's text, lists joined by commas", () => {
    expect(formOf(ticket)).toEqual({
      title: "Split API handlers",
      status: "in-progress",
      type: "refactor",
      priority: "medium",
      created: "2026-10-05",
      branch: "feature/split-api",
      pr: "",
      workstream: "api",
      "depends-on": "FH-12, NG-3",
      tags: "tech-debt, api",
    });
  });

  it("shows a rejected ticket's status as rejected, not its column", () => {
    expect(formOf({ ...ticket, column: "done", rejected: true }).status).toBe(
      "rejected",
    );
  });

  it("gives empty lists as empty text", () => {
    const form = formOf({ ...ticket, dependsOn: [], tags: [] });
    expect(form["depends-on"]).toBe("");
    expect(form.tags).toBe("");
  });
});

describe("changes", () => {
  const saved: TicketForm = formOf(ticket);

  it("is empty when nothing differs, whitespace around values aside", () => {
    expect(changes(saved, saved)).toEqual({});
    expect(
      changes(
        { ...saved, title: "  Split API handlers ", branch: " " },
        {
          ...saved,
          branch: "",
        },
      ),
    ).toEqual({});
  });

  it("lists only the fields that differ, trimmed", () => {
    expect(
      changes({ ...saved, title: " Split handlers ", priority: "high" }, saved),
    ).toEqual({ title: "Split handlers", priority: "high" });
  });

  it("sends lists as arrays, dropping empty items", () => {
    expect(
      changes({ ...saved, "depends-on": "FH-12, , FH-40 ,", tags: "" }, saved),
    ).toEqual({ "depends-on": ["FH-12", "FH-40"], tags: [] });
  });

  it("clears a field set to blank", () => {
    expect(changes({ ...saved, workstream: "" }, saved)).toEqual({
      workstream: "",
    });
  });
});

describe("split", () => {
  it("splits on commas, trims and drops empty items", () => {
    expect(split(" a, b ,,c ")).toEqual(["a", "b", "c"]);
    expect(split("")).toEqual([]);
  });
});

// fieldsText is "Your version" in the conflict dialog (EDIT-7) for a fields
// save.
describe("fieldsText", () => {
  it("puts each changed field on its own line, lists in brackets", () => {
    expect(
      fieldsText({ branch: "feature/split-api", tags: ["api", "ui"] }),
    ).toBe("branch: feature/split-api\ntags: [api, ui]");
  });
});
