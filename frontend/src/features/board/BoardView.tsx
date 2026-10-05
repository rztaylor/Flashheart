import { type KeyboardEvent, useMemo, useRef, useState } from "react";

import {
  type Card,
  COLUMNS,
  type Column,
  type TicketRef,
  type WorkstreamBrief,
} from "../../api/board";
import { Button } from "../../components/Button";
import { EmptyState } from "../../components/EmptyState";
import type { Line } from "../../model/lines";
import { type GridMove, moveInGrid } from "../../model/navigation";
import type { Density } from "../filters/FilterBar";
import { LineLegend } from "./LineLegend";
import { TicketCard } from "./TicketCard";

interface BoardViewProps {
  cards: Card[];
  lines: Map<string, Map<string, Line>>;
  workstreams: Map<string, WorkstreamBrief[]>;
  legend?: { project: string; workstreams: WorkstreamBrief[] };
  projectNames?: Map<string, string>;
  density: Density;
  selected?: TicketRef;
  doneTotal: number;
  doneShown: number;
  doneAll: boolean;
  onDoneAll(all: boolean): void;
  onOpen(ticket: TicketRef): void;
}

const keyMoves: Record<string, GridMove> = {
  ArrowUp: "up",
  ArrowDown: "down",
  ArrowLeft: "left",
  ArrowRight: "right",
  Home: "home",
  End: "end",
};

// BoardView shows real columns in workflow order (VIEW-1). Arrow keys move
// between cards; Enter opens one.
export function BoardView(props: BoardViewProps) {
  const {
    cards,
    lines,
    workstreams,
    legend,
    projectNames,
    density,
    selected,
    doneTotal,
    doneShown,
    doneAll,
    onDoneAll,
    onOpen,
  } = props;
  const [focusedLine, setFocusedLine] = useState("");
  const [cursor, setCursor] = useState({ column: 0, row: 0 });
  const refs = useRef(new Map<string, HTMLButtonElement>());
  const now = new Date();

  const columns = useMemo(() => {
    const grouped = new Map<Column, Card[]>(
      COLUMNS.map((column) => [column.id, []]),
    );
    for (const card of cards) grouped.get(card.column)?.push(card);
    return COLUMNS.map((column) => ({
      ...column,
      cards: grouped.get(column.id) ?? [],
    }));
  }, [cards]);
  const sizes = columns.map((column) => column.cards.length);
  const firstNonEmpty = Math.max(
    0,
    sizes.findIndex((size) => size > 0),
  );
  const active = sizes[cursor.column]
    ? cursor
    : { column: firstNonEmpty, row: 0 };

  const keyOf = (column: number, row: number) => `${column}:${row}`;
  const onKey = (event: KeyboardEvent<HTMLButtonElement>) => {
    const move = keyMoves[event.key];
    if (!move) return;
    event.preventDefault();
    const next = moveInGrid(sizes, active, move);
    setCursor(next);
    refs.current.get(keyOf(next.column, next.row))?.focus();
  };

  const workstreamTitle = (card: Card) =>
    workstreams
      .get(card.project)
      ?.find((workstream) => workstream.slug === card.workstream)?.title ??
    card.workstream;

  return (
    <div className="flex h-full min-h-0 flex-col">
      {legend ? (
        <LineLegend
          workstreams={legend.workstreams}
          lines={lines.get(legend.project) ?? new Map()}
          focused={focusedLine}
          onFocus={setFocusedLine}
        />
      ) : null}
      <div className="board-grid grid min-h-0 flex-1 grid-cols-[repeat(5,minmax(15rem,1fr))] overflow-x-auto px-4 pt-4">
        {columns.map((column, columnIndex) => (
          <section
            key={column.id}
            data-column={column.id}
            aria-labelledby={`column-${column.id}`}
            className="flex min-h-0 flex-col border-l border-rule px-3 first:border-l-0 first:pl-0 last:pr-0"
          >
            <h2
              id={`column-${column.id}`}
              className="flex items-baseline justify-between border-t-[5px] border-rule-strong pt-2 pb-3 text-md station-sign"
            >
              <span>{column.title}</span>
              <span className="text-xs text-ink-muted">
                {column.id === "done" && doneTotal > column.cards.length
                  ? `${column.cards.length} of ${doneTotal}`
                  : column.cards.length}
              </span>
            </h2>
            <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto pb-4">
              {column.cards.length === 0 ? (
                <p className="px-2 py-6 text-center text-xs text-ink-faint">
                  {column.id === "in-progress"
                    ? "Nothing in progress"
                    : "No tickets"}
                </p>
              ) : null}
              {column.cards.map((card, rowIndex) => {
                // One tab stop per column (the cursor's card, or the first
                // card elsewhere) so every scrolling column is reachable;
                // arrow keys move freely across the grid.
                const isActive =
                  active.column === columnIndex
                    ? active.row === rowIndex
                    : rowIndex === 0;
                return (
                  <TicketCard
                    key={`${card.project}/${card.id}/${card.column}`}
                    ref={(node) => {
                      if (node)
                        refs.current.set(keyOf(columnIndex, rowIndex), node);
                      else refs.current.delete(keyOf(columnIndex, rowIndex));
                    }}
                    card={card}
                    line={lines.get(card.project)?.get(card.workstream)}
                    workstreamTitle={
                      card.workstream ? workstreamTitle(card) : undefined
                    }
                    density={density}
                    dimmed={
                      focusedLine !== "" && card.workstream !== focusedLine
                    }
                    showProject={projectNames?.get(card.project)}
                    selected={selected?.id === card.id}
                    tabIndex={isActive ? 0 : -1}
                    now={now}
                    onOpen={() => onOpen({ id: card.id })}
                    onKeyDown={onKey}
                    onFocus={() =>
                      setCursor({ column: columnIndex, row: rowIndex })
                    }
                  />
                );
              })}
              {column.id === "done" &&
              (doneTotal > doneShown || doneAll) &&
              doneTotal > 0 ? (
                <Button
                  variant="quiet"
                  className="mx-auto my-1 text-xs"
                  onClick={() => onDoneAll(!doneAll)}
                >
                  {doneAll ? "Show recent only" : `Show all ${doneTotal}`}
                </Button>
              ) : null}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}

export function NoTickets({ filtered }: { filtered: boolean }) {
  return filtered ? (
    <EmptyState title="No tickets match these filters">
      Clear a filter or the search to see more.
    </EmptyState>
  ) : (
    <EmptyState title="No tickets yet">
      The line is open and nothing is running on it. Agents add tickets here as
      they pick up work.
    </EmptyState>
  );
}
