import { describe, expect, it } from "vitest";

import { ApiError } from "./client";
import {
  addComment,
  blockedReasons,
  conflictOf,
  editComment,
  keysInUse,
  moveTicket,
  patchTicket,
} from "./edit";

function respond(status: number, body: unknown) {
  const calls: { url: string; init?: RequestInit }[] = [];
  const fetcher = async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    return new Response(status === 204 ? null : JSON.stringify(body), {
      status,
    });
  };
  return { fetcher, calls };
}

describe("ticket writes", () => {
  it("sends the hash it read with each edit", async () => {
    const { fetcher, calls } = respond(200, {
      hash: "h2",
      revision: 4,
      warnings: [],
    });
    const saved = await moveTicket(fetcher, "FH-1", "done", "h1");
    expect(saved).toEqual({ hash: "h2", revision: 4, warnings: [] });
    expect(calls[0]?.url).toBe("/api/tickets/FH-1/move");
    expect(calls[0]?.init?.method).toBe("POST");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({
      base: "h1",
      to: "done",
      reason: "",
    });
  });

  it("turns a stale hash into a conflict with the current file", async () => {
    const { fetcher } = respond(409, {
      error: { code: "conflict", message: "changed" },
      current: { hash: "h9", content: "---\nid: FH-1\n---\n" },
    });
    const error = await patchTicket(fetcher, "FH-1", "h1", {
      priority: "high",
    }).catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(ApiError);
    expect(conflictOf(error)).toEqual({
      hash: "h9",
      content: "---\nid: FH-1\n---\n",
    });
    expect(blockedReasons(error)).toBeUndefined();
  });

  it("reads blocking reasons and keys in use from errors", () => {
    const blocked = new ApiError("blocked", 409, "confirm_blocked", {
      error: { code: "confirm_blocked", blockedBy: ["Depends on FH-2"] },
    });
    expect(blockedReasons(blocked)).toEqual(["Depends on FH-2"]);
    const taken = new ApiError("taken", 409, "key_taken", {
      error: { code: "key_taken", inUse: ["FH", "NG"] },
    });
    expect(keysInUse(taken)).toEqual(["FH", "NG"]);
    expect(conflictOf(taken)).toBeUndefined();
  });
});

describe("comments", () => {
  const comment = {
    id: "4e5f6a7b",
    time: "2026-10-13T09:12:00Z",
    by: "Robert",
    byPerson: true,
    state: "open",
    edited: "",
    quoteFrom: "Acceptance criteria 2",
    quote: "Tickets keep their place",
    body: "Not in Done.",
  };

  it("posts a comment with its quote", async () => {
    const { fetcher, calls } = respond(201, {
      comment,
      activityHash: "a2",
      revision: 7,
    });
    const saved = await addComment(fetcher, "FH-1", {
      body: "Not in Done.",
      quote: "Tickets keep their place",
      quoteFrom: "Acceptance criteria 2",
    });
    expect(saved).toEqual({
      hash: "a2",
      revision: 7,
      warnings: [],
      sentTo: "",
    });
    expect(calls[0]?.url).toBe("/api/tickets/FH-1/comments");
    expect(calls[0]?.init?.method).toBe("POST");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({
      body: "Not in Done.",
      quote: "Tickets keep their place",
      quoteFrom: "Acceptance criteria 2",
    });
  });

  it("sends a comment to a session, reading where it went (FH-127)", async () => {
    const { fetcher, calls } = respond(201, {
      comment: { ...comment, sentTo: "claude:s1", delivery: "waiting" },
      activityHash: "a2",
      revision: 7,
      sentTo: "",
      warnings: ["It could not be sent now."],
    });
    const posted = await addComment(fetcher, "FH-1", {
      body: "Not in Done.",
      sendTo: "claude:s1",
    });
    expect(posted).toEqual({
      hash: "a2",
      revision: 7,
      warnings: ["It could not be sent now."],
      sentTo: "",
    });
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({
      body: "Not in Done.",
      quote: "",
      quoteFrom: "",
      sendTo: "claude:s1",
    });
  });

  it("edits a comment on the activity's hash, a stale one a conflict", async () => {
    const ok = respond(200, { comment, activityHash: "a3", revision: 8 });
    expect(
      await editComment(ok.fetcher, "FH-1", "4e5f6a7b", "a2", "New"),
    ).toEqual({ hash: "a3", revision: 8, warnings: [], sentTo: "" });
    expect(ok.calls[0]?.url).toBe("/api/tickets/FH-1/comments/4e5f6a7b");
    expect(ok.calls[0]?.init?.method).toBe("PATCH");
    expect(JSON.parse(String(ok.calls[0]?.init?.body))).toEqual({
      base: "a2",
      body: "New",
    });
    const stale = respond(409, {
      error: { code: "conflict", message: "changed" },
      current: { hash: "a9", content: "# Activity: FH-1\n" },
    });
    const error = await editComment(
      stale.fetcher,
      "FH-1",
      "4e5f6a7b",
      "a2",
      "New",
    ).catch((caught: unknown) => caught);
    expect(conflictOf(error)).toEqual({
      hash: "a9",
      content: "# Activity: FH-1\n",
    });
  });

  it("refuses a response without its comment", async () => {
    const { fetcher } = respond(201, { activityHash: "a2", revision: 7 });
    await expect(addComment(fetcher, "FH-1", { body: "x" })).rejects.toThrow(
      "Comment response was invalid",
    );
  });
});
