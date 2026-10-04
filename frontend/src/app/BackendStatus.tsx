import type { LifecycleState } from "../lifecycle/state";

interface BackendStatusProps {
  state: LifecycleState;
  checking: boolean;
  onCheck(): void;
}

// BackendStatus sits on the signage band and is quiet while healthy: a small
// dot with an accessible name. Trouble shows as words.
export function BackendStatus({
  state,
  checking,
  onCheck,
}: BackendStatusProps) {
  const view = describe(state);
  return (
    <div className="flex items-center gap-2 text-xs text-on-band-muted">
      <span aria-live="polite">
        {checking ? "Checking…" : healthMessage(state)}
      </span>
      <button
        type="button"
        aria-label={view.label}
        title={view.label}
        onClick={onCheck}
        disabled={checking || !view.checkable}
        className="flex h-8 items-center gap-2 rounded-control px-2 transition-colors hover:enabled:bg-band-field focus-visible:outline-on-band disabled:cursor-default"
      >
        <span
          aria-hidden="true"
          className={`size-2 rounded-full ${view.dot}`}
        />
        {view.text ? <span className="text-on-band">{view.text}</span> : null}
      </button>
    </div>
  );
}

function describe(state: LifecycleState) {
  switch (state.phase) {
    case "connected":
      return {
        label: "Backend connected. Check connection",
        dot: "bg-on-band-muted",
        text: "",
        checkable: true,
      };
    case "degraded": {
      const noun = state.failures === 1 ? "heartbeat" : "heartbeats";
      return {
        label: "Backend not responding. Check connection",
        dot: "bg-on-band ring-2 ring-on-band/40",
        text: `Reconnecting · ${state.failures} missed ${noun}`,
        checkable: true,
      };
    }
    case "stopping":
      return {
        label: "Flashheart is stopping",
        dot: "bg-on-band-muted/50",
        text: "Stopping…",
        checkable: false,
      };
    default:
      return {
        label: "Connecting to the backend",
        dot: "bg-on-band-muted/50",
        text: "Connecting…",
        checkable: false,
      };
  }
}

function healthMessage(state: LifecycleState): string {
  if (!state.health) return "";
  const time = state.health.at.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
  return `${state.health.ok ? "Responding" : "Not responding"} · ${time}`;
}
