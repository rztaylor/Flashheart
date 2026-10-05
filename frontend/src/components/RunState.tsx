import type { RunState } from "../api/runs";
import { STATE_LABEL } from "../model/runs";

// RunStateMark draws a run's state as a monochrome shape (ink, shape and
// words, never a hue): Needs you a solid disc, Working a ring around a
// beating dot, Quiet the same ring with a still dot, Waiting an open ring,
// Ended a terminus bar.
export function RunStateMark({
  state,
  size = 12,
  className,
  still,
}: {
  state: RunState;
  size?: number;
  className?: string;
  // still stops the Working beat, for marks in the signage frame.
  still?: boolean;
}) {
  return (
    <svg
      viewBox="0 0 12 12"
      width={size}
      height={size}
      aria-hidden="true"
      data-run-state={state}
      className={["shrink-0", className].filter(Boolean).join(" ")}
    >
      {state === "needs-you" ? (
        <circle cx="6" cy="6" r="5.25" fill="currentColor" />
      ) : null}
      {state === "working" || state === "quiet" || state === "waiting" ? (
        <circle
          cx="6"
          cy="6"
          r="4.75"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          opacity={state === "waiting" ? 0.75 : 1}
        />
      ) : null}
      {state === "working" ? (
        <circle
          cx="6"
          cy="6"
          r="2.25"
          fill="currentColor"
          className={still ? undefined : "run-beat"}
        />
      ) : null}
      {state === "quiet" ? (
        <circle cx="6" cy="6" r="1.75" fill="currentColor" opacity="0.55" />
      ) : null}
      {state === "ended" ? (
        <rect
          x="1.5"
          y="5"
          width="9"
          height="2"
          rx="0.5"
          fill="currentColor"
          opacity="0.6"
        />
      ) : null}
    </svg>
  );
}

// RunStateLabel pairs the mark with its word. Needs you is the one state
// set as an inverted plate, so it reads from across the room.
export function RunStateLabel({
  state,
  tone = "map",
  className,
}: {
  state: RunState;
  // tone "band" draws on the black signage frame.
  tone?: "map" | "band";
  className?: string;
}) {
  if (state === "needs-you") {
    return (
      <span
        className={`inline-flex items-center gap-1 rounded-[3px] px-1.5 leading-4 font-semibold whitespace-nowrap ${
          tone === "band" ? "bg-on-band text-band" : "bg-ink text-ground"
        } ${className ?? ""}`}
      >
        <RunStateMark state={state} size={9} />
        {STATE_LABEL[state]}
      </span>
    );
  }
  return (
    <span
      className={`inline-flex items-center gap-1 whitespace-nowrap ${
        state === "working" ? "text-ink" : "text-ink-muted"
      } ${className ?? ""}`}
    >
      <RunStateMark state={state} size={10} />
      {STATE_LABEL[state]}
    </span>
  );
}
