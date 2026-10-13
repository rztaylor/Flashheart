import { describe, expect, it } from "vitest";

import type { Comment } from "../api/board";
import {
  countRequests,
  excerpt,
  requestChoices,
  requestStateOf,
  requestsOf,
  requestWords,
  sentWords,
} from "./requests";

const comment = (id: string, state: string, byPerson = true): Comment => ({
  id,
  time: "2026-10-13T09:00:00Z",
  by: byPerson ? "Robert" : "claude:abc",
  byShort: byPerson ? "Robert" : "claude:abc",
  byPerson,
  state,
  edited: "",
  quoteFrom: "",
  quote: "",
  body: `Text of ${id}`,
});

describe("change requests (REV-6)", () => {
  const comments = [
    comment("a", "resolved"),
    comment("b", "addressed"),
    comment("c", "change-requested"),
    comment("d", "open"),
    comment("e", "change-requested"),
    comment("f", "change-requested", false),
  ];

  it("lists open requests, then addressed, then resolved, people's only", () => {
    expect(requestsOf(comments).map((c) => c.id)).toEqual(["c", "e", "b", "a"]);
  });

  it("counts what waits on someone", () => {
    expect(countRequests(comments)).toEqual({ open: 2, addressed: 1 });
    expect(countRequests([comment("a", "resolved")])).toEqual({
      open: 0,
      addressed: 0,
    });
  });

  it("says a count in words", () => {
    expect(requestWords({ open: 1, addressed: 0 })).toBe("1 change requested");
    expect(requestWords({ open: 2, addressed: 0 })).toBe("2 changes requested");
    expect(requestWords({ open: 0, addressed: 3 })).toBe("3 addressed");
    expect(requestWords({ open: 2, addressed: 1 })).toBe(
      "2 open · 1 addressed",
    );
    expect(requestWords({ open: 0, addressed: 0 })).toBe("");
    expect(requestWords(undefined)).toBe("");
  });

  it("knows the states a request shows, and nothing else", () => {
    expect(requestStateOf("addressed")).toBe("addressed");
    expect(requestStateOf("open")).toBeUndefined();
    expect(requestStateOf("constructor")).toBeUndefined();
  });

  it("offers people's comments, newest first, open requests checked", () => {
    expect(
      requestChoices(comments).map(({ comment, checked }) => [
        comment.id,
        checked,
      ]),
    ).toEqual([
      ["e", true],
      ["d", false],
      ["c", true],
      ["b", false],
      ["a", false],
    ]);
  });

  it("says where the requests went", () => {
    // The run as the server shortens it (agent-protocol §2).
    expect(sentWords("FH-1", 2, "claude:3f2a9c1e")).toBe(
      "Requested 2 changes on FH-1; sent to claude:3f2a9c1e.",
    );
    expect(sentWords("FH-1", 1, "")).toBe(
      "Requested 1 change on FH-1; the next session on it will see it.",
    );
    expect(sentWords("FH-1", 1, "codex:abc", "Reopened")).toBe(
      "Reopened the change request on FH-1; sent to codex:abc.",
    );
  });

  it("cuts an excerpt to one line", () => {
    expect(excerpt("Rename\n\nthe  button", 90)).toBe("Rename the button");
    expect(excerpt("a".repeat(100), 10)).toBe(`${"a".repeat(9)}…`);
  });
});
