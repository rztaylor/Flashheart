import { useId } from "react";

import type { Metrics } from "../../api/metrics";
import { headlineLabel, headlineTiles } from "../../model/overview";
import { absoluteTime } from "../../model/time";

// HeadlineMetrics is the quiet row under the Overview's title (VIEW-3,
// FH-52): what changed since the user's last change on the board, or in
// the last 24 hours, as equal hairline tiles of a large number over a word.
// Ink only: no colour carries meaning here.
export function HeadlineMetrics({
  metrics,
  now,
}: {
  metrics: Metrics;
  now: Date;
}) {
  const labelId = useId();
  const label = headlineLabel(metrics, now);
  return (
    <section
      aria-labelledby={labelId}
      data-headline-metrics
      className="pt-1 md:pt-0"
    >
      <p id={labelId} className="mb-2 text-sm text-ink-muted">
        {label.text}
        {label.time ? (
          <>
            {" · "}
            <time dateTime={metrics.since} title={absoluteTime(metrics.since)}>
              {label.time}
            </time>
          </>
        ) : null}
      </p>
      <dl className="grid grid-cols-2 gap-2 sm:grid-cols-4 sm:gap-3">
        {headlineTiles(metrics).map((tile) => (
          <div
            key={tile.id}
            data-metric={tile.id}
            className="flex flex-col-reverse items-center gap-1 rounded-card border border-rule bg-card px-3 py-3 text-center"
          >
            <dt className="text-sm text-ink-muted">{tile.word}</dt>
            <dd className="text-3xl leading-none tabular-nums text-ink display-cut">
              {tile.value}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
