import { COLUMNS, type ProjectSummary } from "../../api/board";
import type { Scope } from "../../app/route";
import { Icon } from "../../components/Icon";
import { RouteBar } from "../../components/RouteBar";

interface ProjectRailProps {
  projects: ProjectSummary[];
  scope: Scope;
  onSelect(scope: Scope): void;
}

function total(counts: ProjectSummary["counts"]) {
  return COLUMNS.reduce((sum, column) => sum + counts[column.id], 0);
}

// ProjectRail lists every project with its route bar and trouble counts,
// most recently active first (PRJ-6).
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
  };
  return (
    <nav
      aria-label="Projects"
      className="flex h-full flex-col overflow-y-auto border-r border-rule bg-well"
    >
      <ul className="flex flex-col gap-px p-2">
        <li>
          <RailItem
            active={scope.kind === "all"}
            title="All projects"
            subtitle={`${projects.length} ${projects.length === 1 ? "project" : "projects"}`}
            counts={all.counts}
            blocked={all.blocked}
            repair={all.repair}
            onClick={() => onSelect({ kind: "all" })}
          />
        </li>
        <li aria-hidden="true" className="mx-2 my-1.5 border-t border-rule" />
        {projects.map((project) => (
          <li key={project.name}>
            <RailItem
              active={
                scope.kind === "project" && scope.project === project.name
              }
              title={project.displayName}
              subtitle={
                project.displayName !== project.name ? project.name : undefined
              }
              counts={project.counts}
              blocked={project.stuck}
              repair={project.needsRepair}
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
  subtitle?: string;
  counts: ProjectSummary["counts"];
  blocked: number;
  repair: number;
  onClick(): void;
}

function RailItem({
  active,
  title,
  subtitle,
  counts,
  blocked,
  repair,
  onClick,
}: RailItemProps) {
  const tickets = total(counts);
  return (
    <button
      type="button"
      onClick={onClick}
      title={subtitle}
      aria-current={active ? "page" : undefined}
      className={`flex w-full flex-col gap-1.5 rounded-control px-2.5 py-2 text-left transition-colors ${
        active
          ? "bg-card shadow-[inset_0_0_0_1px_var(--fh-rule)]"
          : "hover:bg-card/60"
      }`}
    >
      <span className="flex items-baseline justify-between gap-2">
        <span
          className={`truncate text-sm ${active ? "font-semibold" : "font-medium"}`}
        >
          {title}
        </span>
        <span className="text-xs text-ink-muted">{tickets}</span>
      </span>
      <RouteBar counts={counts} label={title} />
      <span className="flex items-center gap-3 text-2xs whitespace-nowrap text-ink-muted">
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
            className="flex items-center gap-1 font-semibold text-ink"
            title={`${repair} need repair`}
          >
            <Icon name="repair" size={10} />
            {repair}
            <span className="sr-only"> need repair</span>
          </span>
        ) : null}
      </span>
    </button>
  );
}
