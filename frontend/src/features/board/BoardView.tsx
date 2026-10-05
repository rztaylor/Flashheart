import {
  DndContext,
  type DragEndEvent,
  DragOverlay,
  type DragStartEvent,
  PointerSensor,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  type ComponentProps,
  type KeyboardEvent,
  type ReactNode,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";

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
import { type PaintMode, paintFor, paintKey } from "../../model/paint";
import type { Density } from "../filters/FilterBar";
import { ColourKey } from "./ColourKey";
import { LineLegend } from "./LineLegend";
import { TicketCard } from "./TicketCard";

interface BoardViewProps {
  cards: Card[];
  lines: Map<string, Map<string, Line>>;
  workstreams: Map<string, WorkstreamBrief[]>;
  legend?: { project: string; workstreams: WorkstreamBrief[] };
  projectNames?: Map<string, string>;
  density: Density;
  paint: PaintMode;
  selected?: TicketRef;
  doneTotal: number;
  doneShown: number;
  doneAll: boolean;
  onDoneAll(all: boolean): void;
  onOpen(ticket: TicketRef): void;
  // onMove moves a ticket to another column (EDIT-1); absent when read-only.
  onMove?(card: Card, to: Column): void;
}

const keyMoves: Record<string, GridMove> = {
  ArrowUp: "up",
  ArrowDown: "down",
  ArrowLeft: "left",
  ArrowRight: "right",
  Home: "home",
  End: "end",
};

const columnTitle = (id: string) =>
  COLUMNS.find((column) => column.id === id)?.title ?? id;

const instructions =
  "Press Shift with the left or right arrow to move this ticket to the next column, or drag it with the pointer. Enter opens it.";

// BoardView shows real columns in workflow order (VIEW-1) on the platform
// ground. Arrow keys move between cards; Shift with an arrow moves the
// ticket to the next column, as dragging does (EDIT-1); Enter opens it. When
// a card opens and the panel narrows the board, its column scrolls back into
// view.
export function BoardView(props: BoardViewProps) {
  const {
    cards,
    lines,
    workstreams,
    legend,
    projectNames,
    density,
    paint,
    selected,
    doneTotal,
    doneShown,
    doneAll,
    onDoneAll,
    onOpen,
    onMove,
  } = props;
  const [focusedLine, setFocusedLine] = useState("");
  const [cursor, setCursor] = useState({ column: 0, row: 0 });
  const [dragging, setDragging] = useState<Card | null>(null);
  const refs = useRef(new Map<string, HTMLButtonElement>());
  const refocus = useRef("");
  const now = new Date();
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
  );

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
  const paints = useMemo(
    () => paintKey(cards, paint, new Date()),
    [cards, paint],
  );
  const findCard = (id: string) =>
    [...refs.current.values()].find((element) => element.dataset.ticket === id);

  // Keep the selected card (and so its column) in view as the panel opens
  // beside the board, without moving focus.
  const selectedID = selected?.id;
  // biome-ignore lint/correctness/useExhaustiveDependencies: findCard reads refs.
  useLayoutEffect(() => {
    if (!selectedID) return;
    findCard(selectedID)?.scrollIntoView({
      block: "nearest",
      inline: "nearest",
    });
  }, [selectedID]);

  // A ticket moved with the keyboard keeps focus in its new column.
  // biome-ignore lint/correctness/useExhaustiveDependencies: runs after each change of cards.
  useEffect(() => {
    if (!refocus.current) return;
    const node = findCard(refocus.current);
    if (node) {
      refocus.current = "";
      node.focus();
    }
  }, [cards]);

  const onKey = (card: Card) => (event: KeyboardEvent<HTMLButtonElement>) => {
    if (
      event.shiftKey &&
      onMove &&
      (event.key === "ArrowLeft" || event.key === "ArrowRight")
    ) {
      event.preventDefault();
      const index = COLUMNS.findIndex((column) => column.id === card.column);
      const target = COLUMNS[index + (event.key === "ArrowLeft" ? -1 : 1)];
      if (target) {
        refocus.current = card.id;
        onMove(card, target.id);
      }
      return;
    }
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

  const cardProps = (card: Card) => ({
    card,
    line: lines.get(card.project)?.get(card.workstream),
    workstreamTitle: card.workstream ? workstreamTitle(card) : undefined,
    density,
    paint: paintFor(card, paint, now),
    showProject: projectNames?.get(card.project),
    now,
  });

  const onDragStart = (event: DragStartEvent) =>
    setDragging(cards.find((card) => card.id === event.active.id) ?? null);
  const onDragEnd = (event: DragEndEvent) => {
    setDragging(null);
    const card = cards.find((item) => item.id === event.active.id);
    const to = event.over?.id as Column | undefined;
    if (card && to && to !== card.column) onMove?.(card, to);
  };

  return (
    <DndContext
      sensors={sensors}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onDragCancel={() => setDragging(null)}
      accessibility={{
        screenReaderInstructions: { draggable: instructions },
        announcements: {
          onDragStart: ({ active }) => `Picked up ${active.id}.`,
          onDragOver: ({ active, over }) =>
            over
              ? `${active.id} is over ${columnTitle(String(over.id))}.`
              : undefined,
          onDragEnd: ({ active, over }) =>
            over
              ? `${active.id} dropped in ${columnTitle(String(over.id))}.`
              : `${active.id} put back.`,
          onDragCancel: ({ active }) => `${active.id} put back.`,
        },
      }}
    >
      <div className="platform-ground flex h-full min-h-0 flex-col">
        {(legend && legend.workstreams.length > 0) || paints.length > 0 ? (
          <div className="flex items-center justify-between gap-x-6 overflow-x-auto px-4 pt-3 pb-1 [&>*]:shrink-0">
            {legend ? (
              <LineLegend
                workstreams={legend.workstreams}
                lines={lines.get(legend.project) ?? new Map()}
                focused={focusedLine}
                onFocus={setFocusedLine}
              />
            ) : (
              <span />
            )}
            <ColourKey mode={paint} paints={paints} />
          </div>
        ) : null}
        <div className="board-grid grid min-h-0 flex-1 snap-x snap-mandatory scroll-px-4 grid-cols-[repeat(5,minmax(15rem,1fr))] gap-x-5 overflow-x-auto px-4 pt-4">
          {columns.map((column, columnIndex) => (
            <DropColumn key={column.id} id={column.id} enabled={!!onMove}>
              <h2
                id={`column-${column.id}`}
                className="flex items-baseline justify-between border-t-[5px] border-rule-strong pt-2 pb-3 text-md station-sign"
              >
                <span>{column.title}</span>
                <span className="text-sm font-semibold text-ink">
                  {column.id === "done" && doneTotal > column.cards.length
                    ? `${column.cards.length} of ${doneTotal}`
                    : column.cards.length}
                </span>
              </h2>
              <div className="-mx-2 flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto px-2 pt-0.5 pb-6">
                {column.cards.length === 0 ? (
                  <p className="px-2 py-6 text-center text-xs text-ink-muted">
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
                    <DragCard
                      key={`${card.project}/${card.id}`}
                      enabled={!!onMove}
                      cardRef={(node) => {
                        if (node)
                          refs.current.set(keyOf(columnIndex, rowIndex), node);
                        else refs.current.delete(keyOf(columnIndex, rowIndex));
                      }}
                      {...cardProps(card)}
                      dimmed={
                        focusedLine !== "" && card.workstream !== focusedLine
                      }
                      selected={selected?.id === card.id}
                      ghost={dragging?.id === card.id}
                      tabIndex={isActive ? 0 : -1}
                      onOpen={() => onOpen({ id: card.id })}
                      onKeyDown={onKey(card)}
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
            </DropColumn>
          ))}
        </div>
      </div>
      <DragOverlay dropAnimation={null}>
        {dragging ? (
          <TicketCard
            {...cardProps(dragging)}
            tabIndex={-1}
            onOpen={() => undefined}
            lifted
          />
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}

// DropColumn is a column section that accepts dropped tickets.
function DropColumn({
  id,
  enabled,
  children,
}: {
  id: Column;
  enabled: boolean;
  children: ReactNode;
}) {
  const { setNodeRef, isOver } = useDroppable({ id, disabled: !enabled });
  return (
    <section
      ref={setNodeRef}
      data-column={id}
      aria-labelledby={`column-${id}`}
      className={`-mx-2 flex min-h-0 snap-start flex-col rounded-card px-2 transition-[background-color,box-shadow] ${
        isOver
          ? "bg-card/60 shadow-[inset_0_0_0_2px_var(--fh-rule-strong)]"
          : ""
      }`}
    >
      {children}
    </section>
  );
}

type DragCardProps = ComponentProps<typeof TicketCard> & {
  enabled: boolean;
  cardRef(node: HTMLButtonElement | null): void;
};

// DragCard makes a ticket card a pointer drag source. Keyboard users move
// tickets with Shift and an arrow key instead (EDIT-1).
function DragCard({ enabled, cardRef, ...props }: DragCardProps) {
  const { setNodeRef, listeners, attributes } = useDraggable({
    id: props.card.id,
    disabled: !enabled,
  });
  return (
    <TicketCard
      {...props}
      ref={(node) => {
        setNodeRef(node);
        cardRef(node);
      }}
      dragProps={
        enabled
          ? {
              ...listeners,
              "aria-describedby": attributes["aria-describedby"],
            }
          : undefined
      }
    />
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
