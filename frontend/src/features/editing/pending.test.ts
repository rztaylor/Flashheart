import { describe, expect, it } from "vitest";

import { markSaved, type PendingMoves, serial, settle } from "./pending";

describe("pending moves", () => {
  const pending: PendingMoves = {
    "FH-1": { column: "backlog", after: "FH-3", saved: false, seq: 2 },
  };

  it("are settled only by the save that made them", () => {
    // An older save (seq 1) returning must not settle the newer move.
    expect(markSaved(pending, "FH-1", 1)).toBe(pending);
    expect(markSaved(pending, "FH-1", 2)["FH-1"]?.saved).toBe(true);
    expect(settle(pending, "FH-1", 1)).toBe(pending);
    expect(settle(pending, "FH-1", 2)).toEqual({});
  });
});

describe("serial", () => {
  it("runs saves one after another, in the order asked", async () => {
    const run = serial();
    const log: string[] = [];
    const slow = run(
      () =>
        new Promise<void>((done) =>
          setTimeout(() => {
            log.push("first");
            done();
          }, 20),
        ),
    );
    const fast = run(async () => {
      log.push("second");
    });
    await Promise.all([slow, fast]);
    expect(log).toEqual(["first", "second"]);
  });

  it("keeps going after a failure", async () => {
    const run = serial();
    await expect(
      run(async () => Promise.reject(new Error("no"))),
    ).rejects.toThrow("no");
    await expect(run(async () => "ok")).resolves.toBe("ok");
  });
});
