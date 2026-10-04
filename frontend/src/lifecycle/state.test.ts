import { describe, expect, it } from "vitest";

import {
  initialLifecycleState,
  isTerminal,
  type LifecycleEvent,
  type LifecycleState,
  lifecycleReducer,
} from "./state";

const at = new Date("2026-10-04T12:00:00Z");

function run(...events: LifecycleEvent[]): LifecycleState {
  return events.reduce(lifecycleReducer, initialLifecycleState);
}

describe("lifecycleReducer", () => {
  it("starts connecting and becomes connected", () => {
    expect(initialLifecycleState.phase).toBe("connecting");
    expect(run({ type: "connected" }).phase).toBe("connected");
  });

  it("goes quiet again after a healthy heartbeat", () => {
    const degraded = run(
      { type: "connected" },
      { type: "heartbeat", ok: false, failures: 2, checkedAt: at },
    );
    expect(degraded).toMatchObject({ phase: "degraded", failures: 2 });
    expect(
      lifecycleReducer(degraded, {
        type: "heartbeat",
        ok: true,
        failures: 0,
        checkedAt: at,
      }),
    ).toMatchObject({ phase: "connected", failures: 0, checkedAt: at });
  });

  it("records manual health checks", () => {
    expect(
      run({ type: "connected" }, { type: "health", ok: true, at }).health,
    ).toEqual({ ok: true, at });
  });

  it("keeps the app open with the guard's message when quit is denied", () => {
    const denied = run(
      { type: "connected" },
      { type: "quit-requested" },
      { type: "quit-denied", message: "A change is still being saved." },
    );
    expect(denied).toMatchObject({
      phase: "connected",
      shutdownDenied: "A change is still being saved.",
    });
    expect(
      lifecycleReducer(denied, { type: "dismiss-denial" }).shutdownDenied,
    ).toBeUndefined();
    expect(
      lifecycleReducer(denied, { type: "quit-requested" }).shutdownDenied,
    ).toBeUndefined();
  });

  it("does not let heartbeats interrupt a pending quit", () => {
    const stopping = run({ type: "connected" }, { type: "quit-requested" });
    expect(stopping.phase).toBe("stopping");
    expect(
      lifecycleReducer(stopping, {
        type: "heartbeat",
        ok: true,
        failures: 0,
        checkedAt: at,
      }).phase,
    ).toBe("stopping");
  });

  it("enters terminal states that absorb later events", () => {
    const stopped = run(
      { type: "connected" },
      { type: "quit-requested" },
      { type: "quit-accepted" },
    );
    const lost = run(
      { type: "connected" },
      { type: "server-unavailable", failures: 3 },
    );
    const failed = run({ type: "connect-failed", message: "Unauthorized" });
    expect(stopped.phase).toBe("stopped");
    expect(lost).toMatchObject({ phase: "lost", failures: 3 });
    expect(failed).toMatchObject({ phase: "failed", detail: "Unauthorized" });
    for (const state of [stopped, lost, failed]) {
      expect(isTerminal(state.phase)).toBe(true);
      expect(lifecycleReducer(state, { type: "connected" })).toBe(state);
      expect(
        lifecycleReducer(state, {
          type: "heartbeat",
          ok: true,
          failures: 0,
          checkedAt: at,
        }),
      ).toBe(state);
    }
  });

  it("treats only stopped, lost and failed as terminal", () => {
    expect(isTerminal("connecting")).toBe(false);
    expect(isTerminal("connected")).toBe(false);
    expect(isTerminal("degraded")).toBe(false);
    expect(isTerminal("stopping")).toBe(false);
  });
});
