import type { Column } from "../api/board";

interface RouteBarProps {
  counts: Record<Column, number>;
  label: string;
}

// RouteBar draws a project's progress as a stretch of track between two
// terminal stations: served (review and done) solid, the stretch being
// worked hatched, the rest faint. It draws in the surrounding text colour,
// so it reads on the rail and on its current tile in both themes, and never
// competes with line colours.
export function RouteBar({ counts, label }: RouteBarProps) {
  const served = counts.review + counts.done;
  const working = counts["in-progress"];
  const total = served + working + counts.backlog + counts["up-next"];
  const percent = (value: number) => (total === 0 ? 0 : (value / total) * 100);
  const terminal = (lit: boolean) =>
    `size-2 shrink-0 rounded-full border-2 border-current ${lit ? "bg-current" : ""}`;
  return (
    <div
      role="img"
      aria-label={`${label}: ${served} of ${total} in review or done, ${working} in progress`}
      className="flex items-center"
    >
      <span className={terminal(served > 0)} />
      <span className="relative flex h-1 flex-1 overflow-hidden">
        <span
          aria-hidden="true"
          className="absolute inset-0 bg-current opacity-25"
        />
        <span
          className="relative h-full bg-current"
          style={{ width: `${percent(served)}%` }}
        />
        <span
          className="relative h-full hatched-current"
          style={{ width: `${percent(working)}%` }}
        />
      </span>
      <span className={terminal(total > 0 && served === total)} />
    </div>
  );
}
