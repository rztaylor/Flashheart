import { useId } from "react";

import type { ProjectSummary } from "../../api/board";
import { Icon } from "../../components/Icon";
import { KeyBadge } from "../../components/KeyBadge";
import { attentionClass } from "../../components/NeedsYou";
import { stateToneIcon, stateToneInk } from "../../components/Pill";
import { RunStateMark } from "../../components/RunState";
import { type ProjectRow, projectRows } from "../../model/overview";
import { needsYouPhrase } from "../../model/runs";
import {
  CountBadge,
  quietDisc,
  SectionDisc,
  sectionCardClass,
  sectionHeadClass,
} from "./sectionParts";

// columns is the list's layout once the card is wide enough for a table
// (50rem of its own width, so a ticket panel beside the Overview narrows it
// too): the project, then tickets, the five board columns, blocked, needs
// repair, live and needs you; their least widths (10rem and ten of 3.25rem,
// with the gaps and padding) fit in 50rem. Narrower, a row stacks: the
// project, with its needs repair and Needs you beside the name when not
// zero, over its tickets, blocked and live in words, then the five columns'
// counts over their words. Every class is spelled out whole (@min-[50rem]:)
// so Tailwind finds it.
const columns =
  "grid-cols-5 @min-[50rem]:grid-cols-[minmax(10rem,1.6fr)_repeat(10,minmax(3.25rem,1fr))]";

// ProjectCounts is All projects' list of every project with its counts
// (FH-153, PRJ-6) and live agents (FH-162), in the rail's order, most
// recently active first: what the rail's badges keep in their tooltips,
// side by side. Each row leads to the project's own Overview.
export function ProjectCounts({
  projects,
  hrefFor,
}: {
  projects: ProjectSummary[];
  hrefFor(project: string): string;
}) {
  const titleId = useId();
  if (projects.length === 0) return null;
  const rows = projectRows(projects);
  const [first] = rows;
  return (
    <section
      aria-labelledby={titleId}
      data-projects
      className={`@container ${sectionCardClass}`}
    >
      <h2 className={`${sectionHeadClass} bg-column`}>
        <SectionDisc disc={quietDisc}>
          <Icon name="board" size={15} />
        </SectionDisc>
        <span id={titleId}>Projects</span>
        <CountBadge count={rows.length} />
      </h2>
      {/* The column words, once the rows are a table; each row's name says
          its counts in full. */}
      <div
        aria-hidden="true"
        data-columns
        className={`hidden items-end gap-x-2 border-t border-rule px-4 pt-3 pb-2 text-xs leading-tight font-semibold text-ink-muted heading-cut @min-[50rem]:grid ${columns}`}
      >
        <span>Project</span>
        <span className="text-center">Tickets</span>
        {first?.stages.map((stage) => (
          <span key={stage.column} className="text-center text-balance">
            <Icon
              name={stateToneIcon[stage.tone]}
              size={13}
              className={`mr-1 inline align-[-2px] ${stateToneInk[stage.tone]}`}
            />
            {stage.title}
          </span>
        ))}
        <span className="text-center">
          <Icon
            name={stateToneIcon.blocked}
            size={13}
            className={`mr-1 inline align-[-2px] ${stateToneInk.blocked}`}
          />
          Blocked
        </span>
        <span className="text-center">Needs repair</span>
        <span className="text-center">
          <RunStateMark
            state="working"
            size={11}
            still
            className="mr-1 inline align-[-1px]"
          />
          Live
        </span>
        <span className="text-center">Needs you</span>
      </div>
      <ul className="divide-y divide-rule border-t border-rule">
        {rows.map((row) => (
          <li key={row.name}>
            <ProjectCountsRow row={row} href={hrefFor(row.name)} />
          </li>
        ))}
      </ul>
    </section>
  );
}

// ProjectCountsRow is one project's row. Its link holds only the name
// (with the key and folder name for assistive technology), so its
// accessible name holds the words it shows (WCAG 2.5.3, FH-184); it
// stretches over the row so any of it opens the project, and the counts
// are its description.
function ProjectCountsRow({ row, href }: { row: ProjectRow; href: string }) {
  const summary = useId();
  return (
    <div
      data-project={row.name}
      className={`relative grid items-start gap-x-2 gap-y-2.5 px-4 py-3 transition-colors hover:bg-well has-[:focus-visible]:outline-2 has-[:focus-visible]:-outline-offset-2 has-[:focus-visible]:outline-focus @min-[50rem]:items-center @min-[50rem]:py-2.5 ${columns}`}
    >
      <span id={summary} hidden>
        {row.summary}
      </span>
      <span className="col-span-5 flex min-w-0 items-start gap-2.5 @min-[50rem]:col-span-1 @min-[50rem]:items-center">
        <KeyBadge>
          <span className="display-cut">{row.key}</span>
        </KeyBadge>
        <span className="min-w-0 flex-1">
          <span className="flex items-center gap-1.5">
            <a
              href={href}
              aria-describedby={summary}
              className="truncate text-sm leading-7 font-semibold text-ink outline-none after:absolute after:inset-0"
              title={row.displayName}
            >
              {row.key ? <span className="sr-only">{row.key} </span> : null}
              {row.displayName}
              {row.displayName !== row.name ? (
                <span className="sr-only"> ({row.name})</span>
              ) : null}
            </a>
            <StackedMarks row={row} />
          </span>
          <span className="block text-xs text-ink-muted @min-[50rem]:hidden">
            {row.tickets} {row.tickets === 1 ? "ticket" : "tickets"} ·{" "}
            {row.blocked} blocked ·{" "}
            <span className={row.live > 0 ? undefined : "text-ink-faint"}>
              {row.live} live
            </span>
          </span>
        </span>
      </span>
      <Count className="hidden @min-[50rem]:grid" value={row.tickets} />
      {row.stages.map((stage) => (
        <span
          key={stage.column}
          className="flex min-w-0 flex-col items-center gap-0.5 text-center"
        >
          <Count value={stage.count} />
          <span className="text-2xs leading-tight text-ink-muted @min-[50rem]:hidden">
            {stage.title}
          </span>
        </span>
      ))}
      <Count className="hidden @min-[50rem]:grid" value={row.blocked} />
      <Count
        className="hidden @min-[50rem]:grid"
        value={row.repair}
        mark="repair"
      />
      <Count
        className="hidden @min-[50rem]:grid"
        value={row.live}
        name="live"
      />
      <Count
        className="hidden @min-[50rem]:grid"
        value={row.needsYou}
        mark="needs-you"
      />
    </div>
  );
}

// marks are how a count that needs the user stands out: Needs you in the
// attention colour (the one attention role), needs repair dashed in ink as
// a needs-repair card is (ui-layout.md §7).
const marks = {
  "needs-you": `font-semibold ${attentionClass}`,
  repair: "border-[1.5px] border-dashed border-ink font-semibold text-ink",
} as const;

// Count is one count in a table cell. A zero is a faint dash, so the counts
// that matter stand out (the row's description still says the number).
function Count({
  value,
  mark,
  name,
  className = "grid",
}: {
  value: number;
  mark?: keyof typeof marks;
  // name marks the cell for tests (data-count).
  name?: string;
  className?: string;
}) {
  const marked = mark && value > 0;
  return (
    <span
      data-count={name}
      data-mark={marked ? mark : undefined}
      className={`${className} h-6 min-w-6 place-items-center justify-self-center rounded-full px-1.5 text-sm tabular-nums ${
        marked ? marks[mark] : value > 0 ? "text-ink" : "text-ink-faint"
      }`}
    >
      {value > 0 ? value : "–"}
    </span>
  );
}

// StackedMarks are a stacked row's needs repair and Needs you, beside the
// project's name and only when not zero, marked as in the table (its name
// says them all).
function StackedMarks({ row }: { row: ProjectRow }) {
  const shown = [
    row.repair > 0 ? (
      <span key="repair" className={`${stackedMark} ${marks.repair}`}>
        {row.repair} {row.repair === 1 ? "needs repair" : "need repair"}
      </span>
    ) : null,
    row.needsYou > 0 ? (
      <span key="needs-you" className={`${stackedMark} ${marks["needs-you"]}`}>
        {row.needsYou} {needsYouPhrase(row.needsYou)}
      </span>
    ) : null,
  ];
  return <>{shown}</>;
}

const stackedMark =
  "shrink-0 rounded-full px-1.5 text-xs leading-5 whitespace-nowrap @min-[50rem]:hidden";
