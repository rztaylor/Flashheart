import { describe, expect, it } from "vitest";

import { workstreamSections } from "./workstreamSections";

describe("workstreamSections", () => {
  it("lists the file's sections in order, with the template's missing ones in their place", () => {
    const body =
      "\n# Board UI\n\n## Goal\n\nRender the board.\n\n## Background\n\nWhy.\n\n## Notes\n\nNone.\n";
    expect(workstreamSections(body)).toEqual([
      {
        title: "Goal",
        heading: "Goal",
        markdown: "Render the board.",
        empty: false,
        template: true,
      },
      {
        title: "Background",
        heading: "Background",
        markdown: "Why.",
        empty: false,
        template: false,
      },
      {
        title: "Scope",
        heading: "Scope",
        markdown: "",
        empty: true,
        template: true,
      },
      {
        title: "Success Criteria",
        heading: "Success Criteria",
        markdown: "",
        empty: true,
        template: true,
      },
      {
        title: "Notes",
        heading: "Notes",
        markdown: "None.",
        empty: false,
        template: true,
      },
    ]);
  });

  it("keeps a template section's spelling in the file, and shows an empty one as empty", () => {
    const sections = workstreamSections(
      "# W\n\n## goal\n\n## Scope\n\n```\n## not a heading\n```\n",
    );
    expect(
      sections.map(({ title, heading, empty }) => [title, heading, empty]),
    ).toEqual([
      ["Goal", "goal", true],
      ["Scope", "Scope", false],
      ["Success Criteria", "Success Criteria", true],
      ["Notes", "Notes", true],
    ]);
  });

  it("puts a missing Goal first", () => {
    expect(
      workstreamSections("# W\n\n## Scope\n\nS.\n").map((s) => s.title),
    ).toEqual(["Goal", "Scope", "Success Criteria", "Notes"]);
  });
});
