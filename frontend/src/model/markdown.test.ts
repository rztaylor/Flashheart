import { describe, expect, it } from "vitest";

import { resolveLink, ticketBody } from "./markdown";

describe("resolveLink", () => {
  const from = { project: "alpha", base: "tickets" as const };

  it("opens other tickets for relative markdown links", () => {
    expect(resolveLink("../todo/feat--x.md", from)).toEqual({
      kind: "ticket",
      project: "alpha",
      slug: "feat--x",
    });
    expect(resolveLink("feat--y.md", from)).toEqual({
      kind: "ticket",
      project: "alpha",
      slug: "feat--y",
    });
    expect(
      resolveLink("../ready-to-review/feat--board-columns.md#notes", from),
    ).toEqual({
      kind: "ticket",
      project: "alpha",
      slug: "feat--board-columns",
    });
  });

  it("opens web links externally and refuses other schemes", () => {
    expect(resolveLink("https://example.com/a", from)).toEqual({
      kind: "external",
      href: "https://example.com/a",
    });
    expect(resolveLink("javascript:alert(1)", from)).toEqual({ kind: "none" });
    expect(resolveLink("file:///etc/passwd", from)).toEqual({ kind: "none" });
    expect(resolveLink("#section", from)).toEqual({ kind: "none" });
  });

  it("maps attachment images to the attachment endpoint", () => {
    expect(
      resolveLink("../attachments/feat--a/shot one.png", {
        project: "alpha",
        base: "reviews",
      }),
    ).toEqual({
      kind: "attachment",
      href: "/api/projects/alpha/attachments/feat--a/shot%20one.png",
    });
    expect(resolveLink("../attachments/../todo/x.png", from)).toEqual({
      kind: "none",
    });
  });
});

describe("ticketBody", () => {
  it("drops the title and the sections the panel shows separately, keeping fenced content", () => {
    const body =
      "\n# Title\n\n## Description\n\nText\n\n```\n## Handoff\n```\n\n## Acceptance Criteria\n\n- [ ] one\n\n## Handoff\n\n**Next**\n- x\n\n## Notes\n\nKept.\n";
    const result = ticketBody(body);
    expect(result).not.toContain("# Title");
    expect(result).toContain("## Description");
    expect(result).toContain("```\n## Handoff\n```");
    expect(result).not.toContain("**Next**");
    expect(result).not.toContain("- [ ] one");
    expect(result).toContain("## Notes\n\nKept.");
  });
});
