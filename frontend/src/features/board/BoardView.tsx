import {
  type CollisionDetection,
  DndContext,
  type DragEndEvent,
  type DragMoveEvent,
  DragOverlay,
  type DragStartEvent,
  PointerSensor,
  pointerWithin,
  rectIntersection,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  type ComponentProps,
  type CSSProperties,
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
import type { HideableColumn } from "../../api/preferences";
import { Aside } from "../../components/Aside";
import { Button } from "../../components/Button";
import { EmptySlot, WellHead, wellSurface } from "../../components/ColumnWell";
import { EmptyState } from "../../components/EmptyState";
import { neighbourColumn, shownColumns } from "../../model/columns";
import type { Line } from "../../model/lines";
import { type GridMove, moveInGrid } from "../../model/navigation";
import {
  afterForIndex,
  afterForStep,
  canReorder,
  type DisplaySort,
  displayOrder,
  predecessor,
} from "../../model/order";
import { type PaintMode, paintFor } from "../../model/paint";
import { useNow } from "../../state/useNow";
import type { Density } from "../filters/FilterBar";
import { TicketCard } from "./TicketCard";

interface BoardViewProps {
  cards: Card[];
  lines: Map<string, Map<string, Line>>;
  workstreams: Map<string, WorkstreamBrief[]>;
  projectNames?: Map<string, string>;
  density: Density;
  paint: PaintMode;
  // hiddenColumns are real columns left off the board, and so never a move's
  // target (FH-41).
  hiddenColumns?: HideableColumn[];
  selected?: TicketRef;
  doneTotal: number;
  doneShown: number;
  doneAll: boolean;
  onDoneAll(all: boolean): void;
  onOpen(ticket: TicketRef): void;
  // onMove moves a ticket to another column (EDIT-1) and, with after, to a
  // place in it: directly after that ticket, "" for the top (EDIT-9);
  // absent when read-only.
  onMove?(card: Card, to: Column, after?: string): void;
  // sort is a temporary display sort; null (the default) shows the saved
  // manual order, which only then can be rearranged.
  sort?: DisplaySort;
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
  "Press Shift with the left or right arrow to move this ticket to the next column, or with the up or down arrow to move it within its column, or drag it with the pointer. Enter opens it.";

// Columns are found under the pointer, so a tall card being dragged does not
// pick a neighbouring column; outside every column the nearest one wins.
const collision: CollisionDetection = (args) => {
  const within = pointerWithin(args);
  return within.length > 0 ? within : rectIntersection(args);
};

// A drop's place: an index among the column's shown cards, the dragged card
// left out.
type DropAt = { column: Column; index: number };

// BoardView shows real columns in workflow order (VIEW-1) as rounded wells
// (ui-layout.md §2), each in its saved manual order (EDIT-9). Arrow keys move
// between cards; Shift with Left or Right moves the ticket to the next
// column, as dragging does (EDIT-1), and Shift with Up or Down moves it
// within its column; a drag drops it where a line shows. Enter opens it.
// When a card opens and the panel narrows the board, its column scrolls back
// into view.
export function BoardView(props: BoardViewProps) {
  const {
    cards,
    lines,
    workstreams,
    projectNames,
    density,
    paint,
    hiddenColumns = [],
    selected,
    doneTotal,
    doneShown,
    doneAll,
    onDoneAll,
    onOpen,
    onMove,
    sort = null,
  } = props;
  const reorderable = canReorder(sort);
  // The cursor names its column, so a column hidden or shown does not move
  // it to a neighbour.
  const [cursor, setCursor] = useState<{ column: string; row: number }>({
    column: "",
    row: 0,
  });
  const [dragging, setDragging] = useState<Card | null>(null);
  const [drop, setDrop] = useState<DropAt | null>(null);
  // pointerY follows the pointer while a card is dragged.
  const pointerY = useRef<number | null>(null);
  useEffect(() => {
    if (!dragging) return;
    const follow = (event: PointerEvent) => {
      pointerY.current = event.clientY;
    };
    window.addEventListener("pointermove", follow, { passive: true });
    return () => {
      window.removeEventListener("pointermove", follow);
      pointerY.current = null;
    };
  }, [dragging]);
  const refs = useRef(new Map<string, HTMLButtonElement>());
  const refocus = useRef("");
  const grid = useRef<HTMLDivElement>(null);
  const now = useNow();
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
  );

  const columns = useMemo(() => {
    const grouped = new Map<Column, Card[]>(
      COLUMNS.map((column) => [column.id, []]),
    );
    for (const card of displayOrder(cards, sort))
      grouped.get(card.column)?.push(card);
    return shownColumns(hiddenColumns).map((column) => ({
      id: column.id,
      title: column.title,
      empty: column.id === "in-progress" ? "Nothing in progress" : "No tickets",
      cards: grouped.get(column.id) ?? [],
    }));
  }, [cards, hiddenColumns, sort]);
  const sizes = columns.map((column) => column.cards.length);
  const firstNonEmpty = Math.max(
    0,
    sizes.findIndex((size) => size > 0),
  );
  const cursorColumn = columns.findIndex(
    (column) => column.id === cursor.column,
  );
  const active =
    cursorColumn >= 0 && sizes[cursorColumn]
      ? {
          column: cursorColumn,
          row: Math.min(cursor.row, (sizes[cursorColumn] ?? 1) - 1),
        }
      : { column: firstNonEmpty, row: 0 };
  const moveCursor = (column: number, row: number) =>
    setCursor({ column: columns[column]?.id ?? "", row });

  const keyOf = (column: number, row: number) => `${column}:${row}`;
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
      (event.key === "ArrowUp" || event.key === "ArrowDown")
    ) {
      event.preventDefault();
      if (!reorderable || card.column === "done") return;
      const shown =
        columns.find((column) => column.id === card.column)?.cards ?? [];
      const after = afterForStep(
        shown,
        card,
        event.key === "ArrowUp" ? "up" : "down",
      );
      if (after !== null) {
        refocus.current = card.id;
        onMove(card, card.column, after);
      }
      return;
    }
    if (
      event.shiftKey &&
      onMove &&
      (event.key === "ArrowLeft" || event.key === "ArrowRight")
    ) {
      event.preventDefault();
      const target = neighbourColumn(
        hiddenColumns,
        card.column,
        event.key === "ArrowLeft" ? -1 : 1,
      );
      if (target) {
        refocus.current = card.id;
        onMove(card, target);
      }
      return;
    }
    const move = keyMoves[event.key];
    if (!move) return;
    event.preventDefault();
    const next = moveInGrid(sizes, active, move);
    moveCursor(next.column, next.row);
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

  const shownIn = (id: string) =>
    columns.find((column) => column.id === id)?.cards ?? [];

  // dropAt finds where in the hovered column a drop would land: below every
  // shown card whose middle is above the pointer. Done keeps most recent
  // first, and a display sort has no places, so neither shows one. The
  // pointer is read from the page, not from the drag's delta, which counts
  // the board's own scrolling too.
  const dropAt = (event: DragMoveEvent): DropAt | null => {
    const column = event.over?.id as Column | undefined;
    const pointer = pointerY.current;
    if (!column || column === "done" || !reorderable || pointer === null)
      return null;
    const nodes = grid.current?.querySelectorAll<HTMLElement>(
      `section[data-column="${column}"] [data-ticket]`,
    );
    let index = 0;
    for (const node of nodes ?? []) {
      if (node.dataset.ticket === event.active.id) continue;
      const box = node.getBoundingClientRect();
      if (box.top + box.height / 2 < pointer) index++;
    }
    return { column, index };
  };
  const onDragMove = (event: DragMoveEvent) => {
    const next = dropAt(event);
    setDrop((current) =>
      current?.column === next?.column && current?.index === next?.index
        ? current
        : next,
    );
  };
  const onDragStart = (event: DragStartEvent) => {
    const start = event.activatorEvent as PointerEvent | undefined;
    pointerY.current = start?.clientY ?? null;
    setDragging(cards.find((card) => card.id === event.active.id) ?? null);
  };
  const onDragEnd = (event: DragEndEvent) => {
    const at = drop;
    setDragging(null);
    setDrop(null);
    const card = cards.find((item) => item.id === event.active.id);
    const to = event.over?.id as Column | undefined;
    if (!card || !to) return;
    if (!at || at.column !== to) {
      if (to !== card.column) onMove?.(card, to);
      return;
    }
    const shown = shownIn(to);
    const after = afterForIndex(shown, card, at.index);
    if (to === card.column && after === predecessor(shown, card)) return;
    onMove?.(card, to, after);
  };
  // shownDrop is the drop line's place, unless the drop would change nothing.
  const shownDrop = (() => {
    if (!drop || !dragging) return null;
    const shown = shownIn(drop.column);
    if (
      drop.column === dragging.column &&
      afterForIndex(shown, dragging, drop.index) ===
        predecessor(shown, dragging)
    )
      return null;
    return drop;
  })();

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={collision}
      onDragStart={onDragStart}
      onDragMove={onDragMove}
      onDragEnd={onDragEnd}
      onDragCancel={() => {
        setDragging(null);
        setDrop(null);
      }}
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
      <div className="flex h-full min-h-0 flex-col">
        <div
          ref={grid}
          // The column count is a variable, not an inline template, so the
          // narrow-width rule in index.css (one chosen column) still wins.
          // The grid is the board's one scroller in both directions: columns
          // grow with their cards and scroll down together, and the row
          // stretches every well to the tallest, or to the board's height
          // when every column is short (ui-layout.md §2).
          className="board-grid grid min-h-0 flex-1 snap-x snap-mandatory scroll-pt-16 scroll-px-4 auto-rows-[minmax(max-content,1fr)] grid-cols-[repeat(var(--board-columns),minmax(15rem,1fr))] gap-x-3 overflow-auto px-4 pt-1 pb-4 md:scroll-px-6 md:px-6"
          style={{ "--board-columns": columns.length } as CSSProperties}
        >
          {columns.map((column, columnIndex) => (
            <DropColumn key={column.id} id={column.id} enabled={!!onMove}>
              {/* The title alone names the column region. */}
              <WellHead
                sticky
                titleId={`column-${column.id}`}
                title={column.title}
                count={
                  column.id === "done" && doneTotal > column.cards.length
                    ? `${column.cards.length} of ${doneTotal}`
                    : column.cards.length
                }
              />
              <div className="flex flex-1 flex-col gap-2.5 pt-0.5 pb-3">
                {column.cards.length === 0 ? (
                  <EmptySlot>{column.empty}</EmptySlot>
                ) : null}
                {column.cards.length === 0 && column.id === "review" ? (
                  <Aside
                    placement="review-empty"
                    className="px-1 text-center"
                  />
                ) : null}
                {column.cards.map((card, rowIndex) => {
                  // The drop line sits above the card at the drop's place
                  // (counting cards other than the dragged one).
                  const before =
                    shownDrop?.column === column.id &&
                    card.id !== dragging?.id &&
                    column.cards
                      .slice(0, rowIndex)
                      .filter((item) => item.id !== dragging?.id).length ===
                      shownDrop.index;
                  // One tab stop per column (the cursor's card, or the first
                  // card elsewhere) so every column is reachable;
                  // arrow keys move freely across the grid.
                  const isActive =
                    active.column === columnIndex
                      ? active.row === rowIndex
                      : rowIndex === 0;
                  const ref = (node: HTMLButtonElement | null) => {
                    if (node)
                      refs.current.set(keyOf(columnIndex, rowIndex), node);
                    else refs.current.delete(keyOf(columnIndex, rowIndex));
                  };
                  return (
                    <div
                      key={`${card.project}/${card.id}`}
                      className="relative"
                    >
                      {before ? <DropLine edge="top" /> : null}
                      <DragCard
                        enabled={!!onMove}
                        cardRef={ref}
                        {...cardProps(card)}
                        selected={selected?.id === card.id}
                        ghost={dragging?.id === card.id}
                        tabIndex={isActive ? 0 : -1}
                        onOpen={() => onOpen({ id: card.id })}
                        onKeyDown={onKey(card)}
                        onFocus={() => moveCursor(columnIndex, rowIndex)}
                      />
                    </div>
                  );
                })}
                {shownDrop?.column === column.id &&
                shownDrop.index ===
                  column.cards.filter((item) => item.id !== dragging?.id)
                    .length ? (
                  <div className="relative">
                    <DropLine edge="end" />
                  </div>
                ) : null}
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

// DropLine marks where a dragged ticket will land: above a card, or after
// the last one.
function DropLine({ edge }: { edge: "top" | "end" }) {
  return (
    <span
      aria-hidden="true"
      data-drop-line=""
      className={`pointer-events-none absolute inset-x-1 h-0.5 rounded-full bg-select ${
        edge === "top" ? "-top-1.5" : "-top-1"
      }`}
    />
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
      className={`${wellSurface} snap-start transition-[border-color,box-shadow] ${
        isOver
          ? "border-select shadow-[inset_0_0_0_1px_var(--fh-select)]"
          : "border-rule"
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
      <Aside placement="search" className="mt-3" />
    </EmptyState>
  ) : (
    <EmptyState title="No tickets yet">
      The line is open and nothing is running on it. Agents add tickets here as
      they pick up work.
      <Aside placement="board-empty" className="mt-3" />
    </EmptyState>
  );
}
