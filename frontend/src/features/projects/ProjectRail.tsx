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
// A project with an image of its own shows it on the badge (FH-151); the
// key stays in the name.
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
      className="relative flex h-full flex-col overflow-y-auto bg-rail text-on-rail"
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
              projectKey={project.key}
              subtitle={
                project.displayName !== project.name ? project.name : undefined
              }
              code={project.key}
              image={project.image}
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
  // projectKey is the project's ticket key, named with the project so it
  // is known when an image covers the badge (FH-151).
  projectKey?: string;
  subtitle?: string;
  // code is the project's station code: its ticket key.
  code: ReactNode;
  // image is the project's own image, shown in place of the code.
  image?: string;
  counts: ProjectSummary["counts"];
  blocked: number;
  repair: number;
  runs: RunCounts;
  onClick(): void;
}

// badgeName is a badge's accessible name and tooltip: the project, with
// its key and its folder name when that differs, and the counts the wide
// rail showed. It starts with what the badge shows, the key and then the
// Needs you count, so a voice user can say them (WCAG 2.5.3, FH-184).
function badgeName({
  title,
  projectKey,
  subtitle,
  counts,
  blocked,
  repair,
  runs,
}: Omit<RailBadgeProps, "active" | "code" | "image" | "onClick">) {
  const needsYou =
    runs.needsYouShown > 0
      ? `${runs.needsYouShown} ${needsYouPhrase(runs.needsYouShown)}`
      : "";
  const parts = [
    plural(total(counts), "ticket", "tickets"),
    `${counts["in-progress"]} in progress`,
  ];
  if (blocked > 0) parts.push(`${blocked} blocked`);
  if (repair > 0) parts.push(plural(repair, "needs repair", "need repair"));
  const project = subtitle ? `${title} (${subtitle})` : title;
  if (!projectKey) {
    if (needsYou) parts.push(needsYou);
    return `${project}: ${parts.join(", ")}`;
  }
  const lead = needsYou ? `${projectKey}, ${needsYou}` : projectKey;
  return `${lead}: ${project}, ${parts.join(", ")}`;
}

function RailBadge({ active, code, image, onClick, ...shown }: RailBadgeProps) {
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
          className={`relative grid place-items-center overflow-hidden rounded-control leading-none display-cut ${
            image
              ? "size-8"
              : `h-8 min-w-9 ${long ? "px-1 text-xs" : "px-1.5 text-sm"}`
          } ${
            image
              ? ""
              : active
                ? "bg-rail-active-mark text-on-rail-active-mark"
                : "bg-on-rail/8"
          } ${
            // Needs repair: dashed and hatched in ink, as on its card.
            shown.repair > 0
              ? "outline-[1.5px] outline-offset-2 outline-on-rail outline-dashed"
              : ""
          }`}
        >
          {image ? (
            <img
              src={image}
              alt=""
              data-badge-image
              className="absolute inset-0 size-full object-cover"
            />
          ) : (
            code
          )}
          {shown.repair > 0 ? (
            <span
              aria-hidden="true"
              data-mark="repair"
              className="hatched-strong absolute inset-x-0 top-0 h-1.5"
            />
          ) : null}
        </span>
        {shown.runs.needsYouShown > 0 ? (
          <NeedsYouCount count={shown.runs.needsYouShown} />
        ) : null}
      </span>
    </button>
  );
}
