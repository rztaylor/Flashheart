import type { ReactNode } from "react";

import { COLUMNS, type ProjectSummary, summedRuns } from "../../api/board";
import type { RunCounts } from "../../api/runs";
import type { Scope } from "../../app/route";
import { Icon } from "../../components/Icon";
import { RouteBar } from "../../components/RouteBar";
import { RunStateMark } from "../../components/RunState";

interface ProjectRailProps {
  projects: ProjectSummary[];
  scope: Scope;
  onSelect(scope: Scope): void;
}

function total(counts: ProjectSummary["counts"]) {
  return COLUMNS.reduce((sum, column) => sum + counts[column.id], 0);
}

// ProjectRail is the black signage column beside the board: every project as
// a station code (its ticket key) with its route bar and trouble counts, most
// recently active first (PRJ-6).
export function ProjectRail({ projects, scope, onSelect }: ProjectRailProps) {
  const counts = Object.fromEntries(
    COLUMNS.map((column) => [
      column.id,
      projects.reduce((sum, project) => sum + project.counts[column.id], 0),
    ]),
  ) as ProjectSummary["counts"];
  const all = {
    counts,
    blocked: projects.reduce((sum, project) => sum + project.stuck, 0),
    repair: projects.reduce((sum, project) => sum + project.needsRepair, 0),
    runs: summedRuns(projects),
  };
  return (
    <nav
      aria-label="Projects"
      className="flex h-full flex-col overflow-y-auto bg-band text-on-band"
    >
      <ul className="flex flex-col gap-0.5 p-2">
        <li>
          <RailItem
            active={scope.kind === "all"}
            title="All projects"
            code={<Icon name="board" size={14} />}
            subtitle={`${projects.length} ${projects.length === 1 ? "project" : "projects"}`}
            counts={all.counts}
            blocked={all.blocked}
            repair={all.repair}
            runs={all.runs}
            onClick={() => onSelect({ kind: "all" })}
          />
        </li>
        <li
          aria-hidden="true"
          className="mx-2 my-1.5 border-t border-band-track"
        />
        {projects.map((project) => (
          <li key={project.name}>
            <RailItem
              active={
                scope.kind === "project" && scope.project === project.name
              }
              title={project.displayName}
              code={project.key}
              subtitle={
                project.displayName !== project.name ? project.name : undefined
              }
              counts={project.counts}
              blocked={project.stuck}
              repair={project.needsRepair}
              runs={project.runs}
              onClick={() =>
                onSelect({ kind: "project", project: project.name })
              }
            />
          </li>
        ))}
      </ul>
    </nav>
  );
}

interface RailItemProps {
  active: boolean;
  title: string;
  // code is the project's station code: its ticket key.
  code: ReactNode;
  subtitle?: string;
  counts: ProjectSummary["counts"];
  blocked: number;
  repair: number;
  runs: RunCounts;
  onClick(): void;
}

function RailItem({
  active,
  title,
  code,
  subtitle,
  counts,
  blocked,
  repair,
  runs,
  onClick,
}: RailItemProps) {
  const tickets = total(counts);
  return (
    <button
      type="button"
      onClick={onClick}
      title={`${subtitle ? `${title} (${subtitle})` : title}${runs.needsYou > 0 ? ` — ${runs.needsYou} need${runs.needsYou === 1 ? "s" : ""} you` : ""}`}
      aria-current={active ? "page" : undefined}
      className={`flex w-full items-start gap-2.5 rounded-control px-2 py-2.5 text-left transition-colors focus-visible:outline-on-band max-[90rem]:group-data-[panel=open]/work:justify-center max-[90rem]:group-data-[panel=open]/work:px-0 ${
        active ? "bg-band-field" : "hover:bg-band-field/60"
      }`}
    >
      <span
        aria-hidden="true"
        className={`grid h-6 min-w-8 shrink-0 place-items-center rounded-[3px] px-1 text-xs leading-none font-bold station-sign ${
          active
            ? "bg-on-band text-band"
            : "text-on-band shadow-[inset_0_0_0_1.5px_var(--fh-on-band-muted)]"
        }`}
      >
        {code}
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-1.5 max-[90rem]:group-data-[panel=open]/work:sr-only">
        <span className="flex items-baseline justify-between gap-2">
          <span
            className={`truncate text-sm ${active ? "font-semibold" : "font-medium"}`}
          >
            {title}
          </span>
          <span className="text-xs text-on-band-muted">{tickets}</span>
        </span>
        <RouteBar counts={counts} label={title} />
        <span className="flex items-center gap-3 text-2xs whitespace-nowrap text-on-band-muted">
          {runs.needsYou > 0 ? (
            <span className="flex items-center gap-1 rounded-[3px] bg-on-band px-1 font-semibold text-band">
              <RunStateMark state="needs-you" size={8} />
              {runs.needsYou}
              <span className="sr-only">
                {runs.needsYou === 1 ? " agent needs" : " agents need"} you
              </span>
            </span>
          ) : null}
          <span>{counts["in-progress"]} in progress</span>
          {blocked > 0 ? (
            <span
              className="flex items-center gap-1"
              title={`${blocked} blocked by a dependency or missing reference`}
            >
              <Icon name="diamond" size={10} />
              {blocked}
              <span className="sr-only"> blocked</span>
            </span>
          ) : null}
          {repair > 0 ? (
            <span
              className="flex items-center gap-1 font-semibold text-on-band"
              title={`${repair} need repair`}
            >
              <Icon name="repair" size={10} />
              {repair}
              <span className="sr-only"> need repair</span>
            </span>
          ) : null}
        </span>
      </span>
    </button>
  );
}
