import { describe, expect, it } from "vitest";

import { ApiError } from "../../api/client";
import type { Saved } from "../../api/edit";
import { saveWithBase } from "./precondition";

const saved: Saved = { hash: "h2", revision: 3, warnings: ["note"] };

// Every ticket save from the browser carries the hash it was read at
// (STO-3); a stale one is a conflict with the current file (EDIT-7).
describe("saveWithBase", () => {
  it("sends the base hash and gives back the saved version", async () => {
    const bases: string[] = [];
    const outcome = await saveWithBase("h1", async (base) => {
      bases.push(base);
      return saved;
    });
    expect(bases).toEqual(["h1"]);
    expect(outcome).toEqual({ kind: "saved", saved });
  });

  it("turns a 409 conflict into the current file", async () => {
    const outcome = await saveWithBase("h1", () =>
      Promise.reject(
        new ApiError("changed", 409, "conflict", {
          current: { hash: "h9", content: "---\nid: FH-1\n---\n" },
        }),
      ),
    );
    expect(outcome).toEqual({
      kind: "conflict",
      conflict: { hash: "h9", content: "---\nid: FH-1\n---\n" },
    });
  });

  it("gives any other failure back as it was thrown", async () => {
    const error = new ApiError("refused", 400, "invalid");
    expect(await saveWithBase("h1", () => Promise.reject(error))).toEqual({
      kind: "failed",
      error,
    });
    const thrown = new Error("offline");
    expect(
      await saveWithBase("h1", () => {
        throw thrown;
      }),
    ).toEqual({ kind: "failed", error: thrown });
  });
});
