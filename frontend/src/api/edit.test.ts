import { describe, expect, it } from "vitest";

import { ApiError } from "./client";
import {
  blockedReasons,
  conflictOf,
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
