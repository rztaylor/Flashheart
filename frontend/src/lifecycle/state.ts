// Pure lifecycle state for the single Singleserve session.

export type ConnectionPhase =
  | "connecting"
  | "connected"
  | "degraded"
  | "stopping"
  | "stopped"
  | "lost"
  | "failed";

export interface HealthCheck {
  ok: boolean;
  at: Date;
}

export interface LifecycleState {
  phase: ConnectionPhase;
  failures: number;
  checkedAt?: Date;
  health?: HealthCheck;
  shutdownDenied?: string;
  detail?: string;
}

export type LifecycleEvent =
  | { type: "connected" }
  | { type: "connect-failed"; message: string }
  | { type: "heartbeat"; ok: boolean; failures: number; checkedAt: Date }
  | { type: "health"; ok: boolean; at: Date }
  | { type: "quit-requested" }
  | { type: "quit-denied"; message: string }
  | { type: "quit-accepted" }
  | { type: "dismiss-denial" }
  | { type: "server-unavailable"; failures: number };

export const initialLifecycleState: LifecycleState = {
  phase: "connecting",
  failures: 0,
};

export function isTerminal(phase: ConnectionPhase): boolean {
  return phase === "stopped" || phase === "lost" || phase === "failed";
}

export function lifecycleReducer(
  state: LifecycleState,
  event: LifecycleEvent,
): LifecycleState {
  if (isTerminal(state.phase)) return state;
  switch (event.type) {
    case "connected":
      return { ...state, phase: "connected", failures: 0 };
    case "connect-failed":
      return { phase: "failed", failures: 0, detail: event.message };
    case "heartbeat":
      if (state.phase === "stopping") return state;
      return {
        ...state,
        phase: event.ok ? "connected" : "degraded",
        failures: event.failures,
        checkedAt: event.checkedAt,
      };
    case "health":
      return { ...state, health: { ok: event.ok, at: event.at } };
    case "quit-requested":
      return { ...state, phase: "stopping", shutdownDenied: undefined };
    case "quit-denied":
      return { ...state, phase: "connected", shutdownDenied: event.message };
    case "quit-accepted":
      return { phase: "stopped", failures: 0 };
    case "dismiss-denial":
      return { ...state, shutdownDenied: undefined };
    case "server-unavailable":
      return { phase: "lost", failures: event.failures };
  }
}
