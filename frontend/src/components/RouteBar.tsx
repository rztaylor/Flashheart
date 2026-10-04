interface RouteBarProps {
  counts: {
    todo: number;
    "in-progress": number;
    "ready-to-review": number;
    done: number;
  };
  label: string;
}

// RouteBar draws a project's progress as a stretch of track between two
// terminal stations: served (review and done) in solid ink, the stretch being
// worked hatched, the rest faint. Monochrome so it never competes with line
// colours.
export function RouteBar({ counts, label }: RouteBarProps) {
  const served = counts["ready-to-review"] + counts.done;
  const working = counts["in-progress"];
  const total = served + working + counts.todo;
  const percent = (value: number) => (total === 0 ? 0 : (value / total) * 100);
  return (
    <div
      role="img"
      aria-label={`${label}: ${served} of ${total} in review or done, ${working} in progress`}
      className="flex items-center"
    >
      <span
        className={`size-2 shrink-0 rounded-full border-2 ${served > 0 ? "border-ink bg-ink" : "border-ink-faint bg-ground"}`}
      />
      <span className="flex h-1 flex-1 overflow-hidden bg-rule">
        <span
          className="h-full bg-ink"
          style={{ width: `${percent(served)}%` }}
        />
        <span
          className="hatched-strong h-full"
          style={{ width: `${percent(working)}%` }}
        />
      </span>
      <span
        className={`size-2 shrink-0 rounded-full border-2 ${total > 0 && served === total ? "border-ink bg-ink" : "border-ink-faint bg-ground"}`}
      />
    </div>
  );
}
