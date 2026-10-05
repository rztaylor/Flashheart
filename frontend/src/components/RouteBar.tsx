import type { Column } from "../api/board";

interface RouteBarProps {
  counts: Record<Column, number>;
  label: string;
}

// The bar is drawn in on-band white for the black project rail.
const colours = {
  lit: "border-on-band bg-on-band",
  unlit: "border-on-band-muted bg-band",
  track: "bg-band-track",
  served: "bg-on-band",
  working: "hatched-band",
};

// RouteBar draws a project's progress as a stretch of track between two
// terminal stations: served (review and done) in solid ink, the stretch being
// worked hatched, the rest faint, in on-band white for the black rail.
// Monochrome so it never competes with line colours.
export function RouteBar({ counts, label }: RouteBarProps) {
  const served = counts.review + counts.done;
  const working = counts["in-progress"];
  const total = served + working + counts.backlog + counts["up-next"];
  const percent = (value: number) => (total === 0 ? 0 : (value / total) * 100);
  return (
    <div
      role="img"
      aria-label={`${label}: ${served} of ${total} in review or done, ${working} in progress`}
      className="flex items-center"
    >
      <span
        className={`size-2 shrink-0 rounded-full border-2 ${served > 0 ? colours.lit : colours.unlit}`}
      />
      <span className={`flex h-1 flex-1 overflow-hidden ${colours.track}`}>
        <span
          className={`h-full ${colours.served}`}
          style={{ width: `${percent(served)}%` }}
        />
        <span
          className={`h-full ${colours.working}`}
          style={{ width: `${percent(working)}%` }}
        />
      </span>
      <span
        className={`size-2 shrink-0 rounded-full border-2 ${total > 0 && served === total ? colours.lit : colours.unlit}`}
      />
    </div>
  );
}
