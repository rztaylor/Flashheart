import { describe, expect, it } from "vitest";

import { messageOf } from "./errors";

describe("messageOf", () => {
  it("says what an Error says", () => {
    expect(messageOf(new Error("disk full"), "The request failed")).toBe(
      "disk full",
    );
  });

  it("falls back to the caller's words for anything else", () => {
    expect(messageOf("boom", "The request failed")).toBe("The request failed");
    expect(messageOf(undefined, "Could not load")).toBe("Could not load");
  });

  it("falls back when an Error has no message", () => {
    expect(messageOf(new Error(""), "Could not load")).toBe("Could not load");
  });
});
