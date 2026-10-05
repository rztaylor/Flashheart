import type { WorkstreamBrief } from "../../api/board";
import { LineBullet } from "../../components/LineBullet";
import type { Line } from "../../model/lines";

interface LineLegendProps {
  workstreams: WorkstreamBrief[];
  lines: Map<string, Line>;
  focused: string;
  onFocus(slug: string): void;
}

// LineLegend lists the project's lines; choosing one dims every other card
// rather than hiding it, so the rest of the board stays in view.
export function LineLegend({
  workstreams,
  lines,
  focused,
  onFocus,
}: LineLegendProps) {
  if (workstreams.length === 0) return null;
  return (
    <fieldset className="flex items-center gap-1.5">
      <legend className="sr-only">
        Workstream lines: choose one to highlight it
      </legend>
      {workstreams.map((workstream) => {
        const active = focused === workstream.slug;
        return (
          <button
            key={workstream.slug}
            type="button"
            aria-pressed={active}
            onClick={() => onFocus(active ? "" : workstream.slug)}
            className={`flex items-center gap-2 rounded-full border py-0.5 pr-3 pl-0.5 text-xs shadow-card transition-[color,border-color,opacity] ${
              active
                ? "border-rule-strong bg-card text-ink"
                : "border-rule bg-card text-ink-muted hover:border-ink-muted hover:text-ink"
            } ${focused && !active ? "opacity-50" : ""}`}
          >
            <LineBullet line={lines.get(workstream.slug)} size="md" />
            <span className="font-medium">{workstream.title}</span>
            <span className="text-ink-faint">
              {workstream.done}/{workstream.total}
            </span>
          </button>
        );
      })}
    </fieldset>
  );
}
