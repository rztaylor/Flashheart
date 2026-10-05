import { useMemo, useState } from "react";

import {
  type Card,
  COLUMNS,
  splitReasons,
  type TicketRef,
  type WorkstreamBrief,
} from "../../api/board";
import { Icon } from "../../components/Icon";
import { LineBullet } from "../../components/LineBullet";
import type { Line } from "../../model/lines";
import { runningTime } from "../../model/time";

type SortKey =
  | "title"
  | "project"
  | "column"
  | "type"
  | "priority"
  | "workstream"
  | "state"
  | "criteria"
  | "modified";

interface TableViewProps {
  cards: Card[];
  lines: Map<string, Map<string, Line>>;
  workstreams: Map<string, WorkstreamBrief[]>;
  projectNames?: Map<string, string>;
  selected?: TicketRef;
  onOpen(ticket: TicketRef): void;
}

const priorityRank: Record<string, number> = { high: 0, medium: 1, low: 2 };
const columnRank = (column: string) =>
  COLUMNS.findIndex((item) => item.id === column);
const stateRank = (card: Card) =>
  card.needsRepair.length > 0
    ? 0
    : splitReasons(card.blockedBy).blockers.length > 0
      ? 1
      : card.blocked
        ? 2
        : 3;

function compare(a: Card, b: Card, key: SortKey): number {
  switch (key) {
    case "column":
      return columnRank(a.column) - columnRank(b.column);
    case "priority":
      return (priorityRank[a.priority] ?? 9) - (priorityRank[b.priority] ?? 9);
    case "state":
      return stateRank(a) - stateRank(b);
    case "criteria":
      return (
        a.criteria.done / (a.criteria.total || 1) -
        b.criteria.done / (b.criteria.total || 1)
      );
    case "modified":
      return b.modified.localeCompare(a.modified);
    default:
      return String(a[key]).localeCompare(String(b[key]));
  }
}

// TableView lists tickets in a sortable table (VIEW-5); rows open the card panel.
export function TableView({
  cards,
  lines,
  workstreams,
  projectNames,
  selected,
  onOpen,
}: TableViewProps) {
  const [sort, setSort] = useState<{ key: SortKey; descending: boolean }>({
    key: "column",
    descending: false,
  });
  const rows = useMemo(() => {
    const sorted = [...cards].sort(
      (a, b) =>
        compare(a, b, sort.key) ||
        a.id.localeCompare(b.id, undefined, { numeric: true }),
    );
    return sort.descending ? sorted.reverse() : sorted;
  }, [cards, sort]);
  const columns: { key: SortKey; label: string; className?: string }[] = [
    { key: "title", label: "Ticket" },
    ...(projectNames ? [{ key: "project" as const, label: "Project" }] : []),
    { key: "column", label: "Column" },
    { key: "type", label: "Type" },
    { key: "priority", label: "Priority" },
    { key: "workstream", label: "Workstream" },
    { key: "state", label: "State" },
    { key: "criteria", label: "Criteria", className: "text-right" },
    { key: "modified", label: "Changed", className: "text-right" },
  ];
  const now = new Date();
  return (
    <div className="h-full overflow-auto px-4 pb-10">
      <table className="w-full border-collapse text-sm">
        <thead className="sticky top-0 z-10 bg-ground">
          <tr>
            {columns.map((column) => {
              const active = sort.key === column.key;
              return (
                <th
                  key={column.key}
                  scope="col"
                  aria-sort={
                    active
                      ? sort.descending
                        ? "descending"
                        : "ascending"
                      : "none"
                  }
                  className={`border-b-3 border-rule-strong px-2 pt-3 pb-1.5 text-left text-xs station-sign ${column.className ?? ""}`}
                >
                  <button
                    type="button"
                    onClick={() =>
                      setSort({
                        key: column.key,
                        descending: active ? !sort.descending : false,
                      })
                    }
                    className={`inline-flex items-center gap-1 ${active ? "text-ink" : "text-ink-muted hover:text-ink"}`}
                  >
                    {column.label}
                    {active ? (
                      <Icon
                        name="chevronDown"
                        size={11}
                        className={sort.descending ? "" : "rotate-180"}
                      />
                    ) : null}
                  </button>
                </th>
              );
            })}
          </tr>
        </thead>
        <tbody>
          {rows.map((card) => {
            const isSelected = selected?.id === card.id;
            const title =
              workstreams
                .get(card.project)
                ?.find((item) => item.slug === card.workstream)?.title ??
              card.workstream;
            return (
              <tr
                key={`${card.project}/${card.id}/${card.column}`}
                className={`border-b border-rule ${isSelected ? "bg-well" : "hover:bg-well/60"}`}
              >
                <td className="max-w-[28rem] px-2 py-1.5">
                  <button
                    type="button"
                    onClick={() => onOpen({ id: card.id })}
                    className="flex flex-col text-left"
                  >
                    <span className="font-medium text-ink hover:underline">
                      {card.title}
                    </span>
                    <span className="text-2xs font-semibold tracking-[0.02em] tabular-nums text-ink-muted">
                      {card.id}
                    </span>
                  </button>
                </td>
                {projectNames ? (
                  <td className="px-2 py-1.5 text-ink-muted">
                    {projectNames.get(card.project) ?? card.project}
                  </td>
                ) : null}
                <td className="px-2 py-1.5 whitespace-nowrap">
                  {COLUMNS.find((item) => item.id === card.column)?.title}
                </td>
                <td className="px-2 py-1.5 text-ink-muted">{card.type}</td>
                <td
                  className={`px-2 py-1.5 ${card.priority === "high" ? "font-semibold" : "text-ink-muted"}`}
                >
                  {card.priority}
                </td>
                <td className="px-2 py-1.5">
                  {card.workstream ? (
                    <span className="flex items-center gap-1.5 whitespace-nowrap">
                      <LineBullet
                        line={lines.get(card.project)?.get(card.workstream)}
                        size="sm"
                      />
                      {title}
                    </span>
                  ) : null}
                </td>
                <td className="px-2 py-1.5 whitespace-nowrap">
                  {card.needsRepair.length > 0 ? (
                    <span className="flex items-center gap-1 font-semibold">
                      <Icon name="repair" size={12} />
                      Needs repair
                    </span>
                  ) : card.blocked &&
                    splitReasons(card.blockedBy).blockers.length === 0 ? (
                    <span
                      className="text-ink-faint"
                      title={card.blockedBy
                        .map((reason) => reason.text)
                        .join("\n")}
                    >
                      Waiting
                    </span>
                  ) : card.blocked ? (
                    <span
                      className="flex items-center gap-1"
                      title={card.blockedBy
                        .map((reason) => reason.text)
                        .join("\n")}
                    >
                      <Icon name="diamond" size={12} />
                      Blocked
                    </span>
                  ) : null}
                </td>
                <td className="px-2 py-1.5 text-right text-ink-muted">
                  {card.criteria.total > 0
                    ? `${card.criteria.done}/${card.criteria.total}`
                    : ""}
                </td>
                <td
                  className="px-2 py-1.5 text-right text-ink-muted"
                  title={card.modified}
                >
                  {runningTime(card.modified, now)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
