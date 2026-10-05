import type { PlanItem } from "../api/runs";
import { planStations } from "../model/runs";

// PlanRoute draws an agent's plan as a short route in ink: served steps are
// filled stations, the current step an interchange ring, steps ahead open
// rings on a fainter track. Long plans show as numbers. Line colours stay
// reserved for workstreams, so the route is monochrome.
export function PlanRoute({
  plan,
  done,
  total,
}: {
  plan: PlanItem[];
  done: number;
  total: number;
}) {
  if (total === 0) return null;
  const stations = planStations(plan);
  const label = `Plan ${done} of ${total} done`;
  if (stations.length === 0) {
    return (
      <span className="text-2xs font-semibold text-ink" title={label}>
        {done}/{total}
      </span>
    );
  }
  const gap = 9;
  const width = (stations.length - 1) * gap + 8;
  const current = stations.indexOf("current");
  const travelled = current < 0 ? stations.length - 1 : current;
  return (
    <svg
      role="img"
      aria-label={label}
      viewBox={`0 0 ${width} 8`}
      width={width}
      height={8}
      className="shrink-0 text-ink"
    >
      <line
        x1="4"
        y1="4"
        x2={4 + travelled * gap}
        y2="4"
        stroke="currentColor"
        strokeWidth="1.5"
      />
      <line
        x1={4 + travelled * gap}
        y1="4"
        x2={width - 4}
        y2="4"
        stroke="currentColor"
        strokeWidth="1.5"
        opacity="0.3"
      />
      {stations.map((station, index) => (
        <circle
          // biome-ignore lint/suspicious/noArrayIndexKey: stations are positional.
          key={index}
          cx={4 + index * gap}
          cy="4"
          r={station === "current" ? 3 : 2.5}
          fill={station === "served" ? "currentColor" : "var(--fh-card)"}
          stroke="currentColor"
          strokeWidth={station === "current" ? 1.75 : 1.25}
          opacity={station === "ahead" ? 0.55 : 1}
        />
      ))}
    </svg>
  );
}
