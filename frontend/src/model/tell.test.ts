import { describe, expect, it } from "vitest";
import type { TellSession } from "../api/board";
import {
  commentedWords,
  deliveryWords,
  noSessionReason,
  tellGroups,
  tellLabel,
} from "./tell";

const run = (overrides: Partial<TellSession>): TellSession => ({
  run: "claude:aaaa1111-2222",
  short: "claude:aaaa1111",
  agent: "claude",
  state: "working",
  parent: "",
  agentType: "",
  ...overrides,
});

describe("telling a session now (FH-127)", () => {
  it("groups each session with its subagents, in the server's order", () => {
    const session = run({});
    const explore = run({
      run: "claude:aaaa1111-2222/a1",
      parent: session.run,
      agentType: "Explore",
      state: "waiting",
    });
    const orphan = run({
      run: "claude:bbbb2222/a9",
      short: "claude:bbbb2222",
      parent: "claude:bbbb2222",
    });
    const groups = tellGroups([session, explore, orphan]);
    expect(groups).toEqual([
      { label: "Session claude:aaaa1111", session, subagents: [explore] },
      { label: "Subagents", subagents: [orphan] },
    ]);
    expect(tellLabel(session)).toBe("claude:aaaa1111 · Working");
    expect(tellLabel(explore)).toBe("Explore subagent · Waiting");
    expect(tellLabel(orphan)).toBe("Subagent · Working");
    expect(tellGroups([])).toEqual([]);
  });

  it("words why there is no session, where a comment went, and its delivery", () => {
    expect(noSessionReason("FH-42")).toBe("No live session holds FH-42 now.");
    expect(commentedWords("FH-42", "")).toBe("Commented on FH-42.");
    expect(commentedWords("FH-42", "claude:aaaa1111-2222")).toBe(
      "Commented on FH-42 and sent it to claude:aaaa1111.",
    );
    expect(deliveryWords("waiting", "claude:aaaa1111")).toBe(
      "Waiting for claude:aaaa1111 to take it with its next prompt or tool call.",
    );
    expect(deliveryWords("delivered", "claude:aaaa1111")).toBe(
      "Delivered to claude:aaaa1111",
    );
    expect(deliveryWords("missed", "claude:aaaa1111")).toBe(
      "Not delivered: claude:aaaa1111 ended before taking it. It stays on the ticket.",
    );
  });
});
