import { Button } from "../components/Button";
import type { LifecycleState } from "../lifecycle/state";

interface BackendStatusProps {
  state: LifecycleState;
  checking: boolean;
  onCheck(): void;
}

// BackendStatus is quiet while healthy: a dot with an accessible name.
export function BackendStatus({
  state,
  checking,
  onCheck,
}: BackendStatusProps) {
  const view = describe(state);
  return (
    <div className="flex items-center gap-2 text-sm">
      <span aria-live="polite" className="text-text-muted">
        {checking ? "Checking…" : healthMessage(state)}
      </span>
      <Button
        variant="quiet"
        aria-label={view.label}
        title={view.label}
        onClick={onCheck}
        disabled={checking || !view.checkable}
      >
        <span
          aria-hidden="true"
          className={`size-2.5 rounded-full ${view.dot}`}
        />
        {view.text ? <span>{view.text}</span> : null}
      </Button>
    </div>
  );
}

function describe(state: LifecycleState) {
  switch (state.phase) {
    case "connected":
      return {
        label: "Backend connected. Check connection",
        dot: "bg-success",
        text: "",
        checkable: true,
      };
    case "degraded": {
      const noun = state.failures === 1 ? "heartbeat" : "heartbeats";
      return {
        label: "Backend not responding. Check connection",
        dot: "bg-warning",
        text: `Reconnecting · ${state.failures} missed ${noun}`,
        checkable: true,
      };
    }
    case "stopping":
      return {
        label: "Flashheart is stopping",
        dot: "bg-text-muted",
        text: "Stopping…",
        checkable: false,
      };
    default:
      return {
        label: "Connecting to the backend",
        dot: "bg-text-muted",
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
