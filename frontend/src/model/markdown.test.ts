import { describe, expect, it } from "vitest";

import {
  linkTicketIds,
  resolveLink,
  ticketBody,
  ticketPageHref,
  ticketRefs,
} from "./markdown";

const context = { project: "alpha", ticket: "AL-2" };

describe("resolveLink", () => {
  it("opens tickets for links into ticket folders", () => {
    expect(
      resolveLink("../AL-4-drag-and-drop/AL-4-drag-and-drop.md", context),
    ).toEqual({ kind: "ticket", id: "AL-4" });
    expect(resolveLink("AL-2-board-columns.md", context)).toEqual({
      kind: "ticket",
      id: "AL-2",
    });
    expect(resolveLink("../BE-1-hello/BE-1-hello.md#notes", context)).toEqual({
      kind: "ticket",
      id: "BE-1",
    });
    // A file named after a different id than its folder is not a ticket link.
    expect(resolveLink("../BE-1-hello/AL-3-x.md", context)).toEqual({
      kind: "none",
    });
    expect(resolveLink("review.md", context)).toEqual({ kind: "none" });
    expect(resolveLink("ticket.md", context)).toEqual({ kind: "none" });
    expect(resolveLink("#ticket-AL-9", context)).toEqual({
      kind: "ticket",
      id: "AL-9",
    });
  });

  it("opens web links externally and refuses other schemes", () => {
    expect(resolveLink("https://example.com/a", context)).toEqual({
      kind: "external",
      href: "https://example.com/a",
    });
    expect(resolveLink("javascript:alert(1)", context)).toEqual({
      kind: "none",
    });
    expect(resolveLink("file:///etc/passwd", context)).toEqual({
      kind: "none",
    });
    expect(resolveLink("#section", context)).toEqual({ kind: "none" });
    expect(resolveLink("/etc/passwd", context)).toEqual({ kind: "none" });
  });

  it("maps files to the ticket files endpoint", () => {
    expect(resolveLink("files/shot one.png", context)).toEqual({
      kind: "attachment",
      href: "/api/projects/alpha/tickets/AL-2/files/shot%20one.png",
    });
    expect(resolveLink("../AL-4-x/files/b.png", context)).toEqual({
      kind: "attachment",
      href: "/api/projects/alpha/tickets/AL-4/files/b.png",
    });
    expect(resolveLink("files/../../x.png", context)).toEqual({ kind: "none" });
  });
});

describe("linkTicketIds", () => {
  it("links known ticket ids in text, outside code and existing links", () => {
    const tree = {
      type: "root",
      children: [
        {
          type: "paragraph",
          children: [
            { type: "text", value: "See AL-4 and BE-1, not XX-1 or AL-4x." },
            { type: "inlineCode", value: "AL-4" },
            {
              type: "link",
              url: "x",
              children: [{ type: "text", value: "AL-5" }],
            },
          ],
        },
      ],
    };
    linkTicketIds(new Set(["AL", "BE"]))(tree);
    const paragraph = tree.children[0] as {
      children: { type: string; value?: string; url?: string }[];
    };
    expect(paragraph.children.map((node) => node.type)).toEqual([
      "text",
      "link",
      "text",
      "link",
      "text",
      "inlineCode",
      "link",
    ]);
    expect(paragraph.children[1]).toMatchObject({
      type: "link",
      url: "#ticket-AL-4",
    });
    expect(paragraph.children[3]).toMatchObject({
      type: "link",
      url: "#ticket-BE-1",
    });
    expect(paragraph.children[4]).toMatchObject({
      type: "text",
      value: ", not XX-1 or AL-4x.",
    });
  });
});

describe("ticketPageHref", () => {
  it("names a ticket's full page by id", () => {
    expect(ticketPageHref("FH-42")).toBe("#/ticket/FH-42");
  });
});

describe("ticketRefs", () => {
  it("splits plain text into text and ticket ids of known keys", () => {
    expect(
      ticketRefs(
        "Depends on AL-4 and XX-1, then BE-12.",
        new Set(["AL", "BE"]),
      ),
    ).toEqual([
      "Depends on ",
      { id: "AL-4" },
      " and XX-1, then ",
      { id: "BE-12" },
      ".",
    ]);
  });

  it("leaves text without ids, and ids glued to other words, alone", () => {
    expect(ticketRefs("No refs here", new Set(["AL"]))).toEqual([
      "No refs here",
    ]);
    expect(ticketRefs("AL-4x and xAL-4", new Set(["AL"]))).toEqual([
      "AL-4x and xAL-4",
    ]);
    expect(ticketRefs("AL-4", new Set(["AL"]))).toEqual([{ id: "AL-4" }]);
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
