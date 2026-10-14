import type { ReactNode } from "react";

import { COLUMNS, type ProjectSummary } from "../../api/board";
import { noRuns, type RunCounts } from "../../api/runs";
import { Icon } from "../../components/Icon";
import { NeedsYouCount } from "../../components/NeedsYou";
import { needsYouPhrase } from "../../model/runs";
import type { Scope } from "../../model/scopes";

interface ProjectRailProps {
  projects: ProjectSummary[];
  // runs are every project's run counts, from the server (All projects).
  runs?: RunCounts;
  scope: Scope;
  onSelect(scope: Scope): void;
}

function total(counts: ProjectSummary["counts"]) {
  return COLUMNS.reduce((sum, column) => sum + counts[column.id], 0);
}

function plural(count: number, one: string, many: string) {
  return `${count} ${count === 1 ? one : many}`;
}

// ProjectRail is the frame's slim column beside the view (FH-150): All
// projects, then every project's key badge, most recently active first
// (PRJ-6, ui-layout.md §1). A badge shows only what needs the user, its
// Needs you count and a repair mark; its name and tooltip carry the rest.
export function ProjectRail({
  projects,
  runs,
  scope,
  onSelect,
}: ProjectRailProps) {
  const counts = Object.fromEntries(
    COLUMNS.map((column) => [
      column.id,
      projects.reduce((sum, project) => sum + project.counts[column.id], 0),
    ]),
  ) as ProjectSummary["counts"];
  return (
    <nav
      aria-label="Projects"
      className="flex h-full flex-col overflow-y-auto bg-rail text-on-rail"
    >
      <ul className="flex flex-col gap-1 px-1 py-2">
        <li>
          <RailBadge
            active={scope.kind === "all"}
            title="All projects"
            subtitle={plural(projects.length, "project", "projects")}
            code={<Icon name="board" size={14} />}
            counts={counts}
            blocked={projects.reduce(
              (sum, project) => sum + project.blocked,
              0,
            )}
            repair={projects.reduce(
              (sum, project) => sum + project.needsRepair,
              0,
            )}
            runs={runs ?? noRuns}
            onClick={() => onSelect({ kind: "all" })}
          />
        </li>
        <li
          aria-hidden="true"
          className="mx-3 my-1 border-t border-rail-rule"
        />
        {projects.map((project) => (
          <li key={project.name}>
            <RailBadge
              active={
                scope.kind === "project" && scope.project === project.name
              }
              title={project.displayName}
              subtitle={
                project.displayName !== project.name ? project.name : undefined
              }
              code={project.key}
              counts={project.counts}
              blocked={project.blocked}
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

interface RailBadgeProps {
  active: boolean;
  title: string;
  subtitle?: string;
  // code is the project's station code: its ticket key.
  code: ReactNode;
  counts: ProjectSummary["counts"];
  blocked: number;
  repair: number;
  runs: RunCounts;
  onClick(): void;
}

// badgeName is a badge's accessible name and tooltip: the project and the
// counts the wide rail showed.
function badgeName({
  title,
  subtitle,
  counts,
  blocked,
  repair,
  runs,
}: Omit<RailBadgeProps, "active" | "code" | "onClick">) {
  const parts = [
    plural(total(counts), "ticket", "tickets"),
    `${counts["in-progress"]} in progress`,
  ];
  if (blocked > 0) parts.push(`${blocked} blocked`);
  if (repair > 0) parts.push(plural(repair, "needs repair", "need repair"));
  if (runs.needsYou > 0)
    parts.push(`${runs.needsYou} ${needsYouPhrase(runs.needsYou, true)}`);
  return `${subtitle ? `${title} (${subtitle})` : title}: ${parts.join(", ")}`;
}

function RailBadge({ active, code, onClick, ...shown }: RailBadgeProps) {
  const name = badgeName(shown);
  const long = typeof code === "string" && code.length > 3;
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={name}
      title={name}
      aria-current={active ? "page" : undefined}
      className={`grid h-12 w-full place-items-center rounded-card border transition-colors ${
        active
          ? "border-rail-rule bg-rail-active shadow-card"
          : "border-transparent hover:bg-on-rail/6"
      }`}
    >
      <span className="relative">
        <span
          className={`relative grid h-8 min-w-9 place-items-center overflow-hidden rounded-control leading-none display-cut ${
            long ? "px-1 text-xs" : "px-1.5 text-sm"
          } ${
            active
              ? "bg-rail-active-mark text-on-rail-active-mark"
              : "bg-on-rail/8"
          } ${
            // Needs repair: dashed and hatched in ink, as on its card.
            shown.repair > 0
              ? "outline-[1.5px] outline-offset-2 outline-on-rail outline-dashed"
              : ""
          }`}
        >
          {shown.repair > 0 ? (
            <span
              aria-hidden="true"
              data-mark="repair"
              className="hatched-strong absolute inset-x-0 top-0 h-1.5"
            />
          ) : null}
          {code}
        </span>
        {shown.runs.needsYou > 0 ? (
          <NeedsYouCount count={shown.runs.needsYou} />
        ) : null}
      </span>
    </button>
  );
}
