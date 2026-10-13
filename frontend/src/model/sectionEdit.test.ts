import { describe, expect, it } from "vitest";

import {
  criteriaOf,
  criteriaText,
  fieldLines,
  sameSection,
  sectionSource,
  sectionText,
  titleLine,
} from "./sectionEdit";

const body = [
  "# Edit in place",
  "",
  "## Description",
  "",
  "",
  "  Indented first line.",
  "Second line.  ",
  "",
  "```md",
  "## Not a heading",
  "```",
  "",
  "## Acceptance Criteria",
  "",
  "- [x] One",
  "- [ ] Two  ",
  "",
  "## Context",
  "",
  "## Notes",
  "",
  "None.",
  "",
].join("\n");

describe("sectionSource", () => {
  it("reads a section as written, without the blank lines around it", () => {
    expect(sectionSource(body, "Description")).toBe(
      "  Indented first line.\nSecond line.  \n\n```md\n## Not a heading\n```",
    );
    expect(sectionSource(body, "acceptance criteria")).toBe(
      "- [x] One\n- [ ] Two",
    );
  });

  it("reads an empty or missing section as empty", () => {
    expect(sectionSource(body, "Context")).toBe("");
    expect(sectionSource(body, "Test Plan")).toBe("");
  });

  it("reads Windows line endings as plain ones", () => {
    expect(sectionSource(body.replace(/\n/g, "\r\n"), "Notes")).toBe("None.");
  });
});

describe("sameSection", () => {
  it("ignores the blank lines around a section and trailing whitespace, as the server does", () => {
    expect(sectionText("\n\n  Text.  \n\n")).toBe("  Text.");
    expect(sameSection("Text.", "\nText.\n\n")).toBe(true);
    expect(sameSection("Text.", " Text.")).toBe(false);
    expect(sameSection("A\nB", "A\n\nB")).toBe(false);
  });
});

describe("the part of a file an edit changed", () => {
  const file = [
    "---",
    "id: FH-1",
    "tags:",
    "  - ui",
    "  - editor",
    "priority: high",
    "---",
    "",
    "```",
    "# Not the title",
    "```",
    "# The title",
    "",
  ].join("\n");

  it("finds the title line outside code", () => {
    expect(titleLine(file)).toBe("# The title");
    expect(titleLine("no title")).toBe("");
  });

  it("finds a field's lines, a block list's items too", () => {
    expect(fieldLines(file, "priority")).toBe("priority: high");
    expect(fieldLines(file, "tags")).toBe("tags:\n  - ui\n  - editor");
    expect(fieldLines(file, "branch")).toBe("");
    expect(fieldLines("# no frontmatter", "id")).toBe("");
  });
});

describe("criteria", () => {
  it("reads a plain task list as criteria", () => {
    expect(criteriaOf("- [x] One\n\n* [ ] Two  \n- [X] Three")).toEqual([
      { text: "One", done: true },
      { text: "Two", done: false },
      { text: "Three", done: true },
    ]);
    expect(criteriaOf("")).toEqual([]);
  });

  it("leaves a section with other text to the markdown editor", () => {
    expect(criteriaOf("Before.\n\n- [ ] One")).toBeUndefined();
    expect(criteriaOf("- [ ] One\n  - [ ] Nested")).toBeUndefined();
    expect(criteriaOf("- One")).toBeUndefined();
  });

  it("writes criteria one to a line, dropping empty ones", () => {
    expect(
      criteriaText([
        { text: "One", done: true },
        { text: "  ", done: false },
        { text: "Two\nlines ", done: false },
      ]),
    ).toBe("- [x] One\n- [ ] Two lines");
  });
});
