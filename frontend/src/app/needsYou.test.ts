import { describe, expect, it } from "vitest";

import { pressNeedsYou } from "./needsYou";
import type { Scope } from "./route";

const all: Scope = { kind: "all" };
const alpha: Scope = { kind: "project", project: "alpha" };
const beta: Scope = { kind: "project", project: "beta" };

describe("pressNeedsYou (FH-44)", () => {
  it("turns on in All projects, remembering the project it came from", () => {
    expect(pressNeedsYou({ scope: alpha, view: "board" }, false)).toEqual({
      on: true,
      go: { scope: all, view: "board", ticket: undefined },
      cameFrom: alpha,
    });
    expect(pressNeedsYou({ scope: all, view: "board" }, false)).toEqual({
      on: true,
      go: { scope: all, view: "board", ticket: undefined },
      cameFrom: undefined,
    });
  });

  it("keeps the Table, and opens the Board from any other view", () => {
    expect(pressNeedsYou({ scope: alpha, view: "table" }, false).go?.view).toBe(
      "table",
    );
    for (const view of ["agents", "workstreams", "archive", "ticket"] as const)
      expect(pressNeedsYou({ scope: alpha, view }, false).go?.view).toBe(
        "board",
      );
  });

  it("turns off back in the project it came from", () => {
    expect(pressNeedsYou({ scope: all, view: "table" }, true, alpha)).toEqual({
      on: false,
      go: { scope: alpha },
      cameFrom: undefined,
    });
  });

  it("turns off in place once another scope was chosen", () => {
    expect(pressNeedsYou({ scope: beta, view: "board" }, true, alpha)).toEqual({
      on: false,
      go: undefined,
      cameFrom: undefined,
    });
    expect(pressNeedsYou({ scope: all, view: "board" }, true)).toEqual({
      on: false,
      go: undefined,
      cameFrom: undefined,
    });
  });
});
