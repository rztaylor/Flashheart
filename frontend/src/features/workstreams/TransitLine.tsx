import {
  DndContext,
  type DragEndEvent,
  PointerSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  arrayMove,
  horizontalListSortingStrategy,
  SortableContext,
  useSortable,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import {
  type CSSProperties,
  type KeyboardEvent,
  type ReactNode,
  useEffect,
  useRef,
  useState,
} from "react";

import {
  COLUMNS,
  type TicketRef,
  type Workstream,
  type WorkstreamTicket,
} from "../../api/board";
import { Icon } from "../../components/Icon";
import type { Line } from "../../model/lines";

interface TransitLineProps {
  workstream: Workstream;
  line?: Line;
  onOpen(ticket: TicketRef): void;
  // onReorder saves a new station order (EDIT-4); absent when read-only.
  onReorder?(ids: string[]): void;
}

const served = (ticket: WorkstreamTicket) =>
  ticket.column === "review" ||
  ticket.column === "done" ||
  ticket.column === "archived";

function stationState(ticket: WorkstreamTicket, next: string) {
  if (ticket.missing) return "missing";
  if (served(ticket)) return "served";
  if (ticket.id === next) return "next";
  return "pending";
}

const columnTitle = (column: WorkstreamTicket["column"]) =>
  column === "archived"
    ? "Archived"
    : (COLUMNS.find((item) => item.id === column)?.title ?? "Missing");

type Track = "served" | "ahead" | "suspended";

// TransitLine draws a workstream as a transit line (VIEW-4): its tickets are
// stations in order. Track already travelled is solid line colour; the route
// ahead is lighter; only suspended service is dashed: the whole line when
// its own workstream dependencies are unmet, or the track into a station
// held by something outside the line. The next stop is an interchange ring.
export function TransitLine({
  workstream,
  line,
  onOpen,
  onReorder,
}: TransitLineProps) {
  const colour = line ? `var(--fh-line-${line.colour})` : "var(--fh-ink-faint)";
  const scroller = useRef<HTMLOListElement>(null);
  const [more, setMore] = useState(false);
  const refocus = useRef("");
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
  );
  const ids = workstream.tickets.map(
    (ticket, index) => `${ticket.id}#${index}`,
  );

  // A station moved with the keyboard keeps focus at its new place.
  // biome-ignore lint/correctness/useExhaustiveDependencies: runs after the order changes.
  useEffect(() => {
    if (!refocus.current) return;
    scroller.current
      ?.querySelector<HTMLButtonElement>(`[data-station="${refocus.current}"]`)
      ?.focus();
    refocus.current = "";
  }, [workstream.tickets]);

  const order = (from: number, to: number) =>
    arrayMove(
      workstream.tickets.map((ticket) => ticket.id),
      from,
      to,
    );
  const onKey =
    (index: number) => (event: KeyboardEvent<HTMLButtonElement>) => {
      if (!onReorder || !event.shiftKey) return;
      const step =
        event.key === "ArrowLeft" ? -1 : event.key === "ArrowRight" ? 1 : 0;
      const to = index + step;
      if (step === 0 || to < 0 || to >= workstream.tickets.length) return;
      event.preventDefault();
      refocus.current = workstream.tickets[index]?.id ?? "";
      onReorder(order(index, to));
    };
  const onDragEnd = (event: DragEndEvent) => {
    const from = ids.indexOf(String(event.active.id));
    const to = event.over ? ids.indexOf(String(event.over.id)) : -1;
    if (onReorder && from >= 0 && to >= 0 && from !== to)
      onReorder(order(from, to));
  };

  useEffect(() => {
    const element = scroller.current;
    if (!element) return;
    const update = () =>
      setMore(
        element.scrollLeft + element.clientWidth < element.scrollWidth - 4,
      );
    update();
    const observer = new ResizeObserver(update);
    observer.observe(element);
    element.addEventListener("scroll", update, { passive: true });
    return () => {
      observer.disconnect();
      element.removeEventListener("scroll", update);
    };
  }, []);

  if (workstream.tickets.length === 0) {
    return (
      <p className="py-4 text-xs text-ink-faint">
        No tickets on this line yet.
      </p>
    );
  }

  const trackInto = (ticket: WorkstreamTicket | undefined): Track => {
    if (!ticket) return "ahead";
    if (served(ticket)) return "served";
    if (workstream.suspended || ticket.held || ticket.missing)
      return "suspended";
    return "ahead";
  };
  const trackStyle = (track: Track): CSSProperties => {
    switch (track) {
      case "served":
        return { background: colour };
      case "suspended":
        return {
          backgroundImage: `repeating-linear-gradient(90deg, ${colour} 0 8px, transparent 8px 14px)`,
        };
      default:
        return { background: colour, opacity: 0.35 };
    }
  };

  return (
    <DndContext
      sensors={sensors}
      onDragEnd={onDragEnd}
      accessibility={{
        screenReaderInstructions: {
          draggable:
            "Press Shift with the left or right arrow to move this station earlier or later on the line, or drag it with the pointer.",
        },
      }}
    >
      <SortableContext
        items={ids}
        strategy={horizontalListSortingStrategy}
        disabled={!onReorder}
      >
        <div className="relative">
          <ol
            ref={scroller}
            className="flex overflow-x-auto pt-7 pb-2"
            aria-label={`${workstream.title} stations`}
          >
            {workstream.tickets.map((ticket, index) => {
              const state = stationState(ticket, workstream.next);
              const first = index === 0;
              const last = index === workstream.tickets.length - 1;
              return (
                <SortableStation
                  // biome-ignore lint/suspicious/noArrayIndexKey: a hand-edited tickets list may repeat an id; position is the identity.
                  key={`${ticket.id}-${index}`}
                  id={ids[index] ?? ""}
                  enabled={!!onReorder}
                >
                  {(drag) => (
                    <>
                      <span
                        aria-hidden="true"
                        className="absolute top-[11px] left-0 h-1.5 w-1/2"
                        style={
                          first ? undefined : trackStyle(trackInto(ticket))
                        }
                      />
                      <span
                        aria-hidden="true"
                        className="absolute top-[11px] right-0 h-1.5 w-1/2"
                        style={
                          last
                            ? undefined
                            : trackStyle(
                                trackInto(workstream.tickets[index + 1]),
                              )
                        }
                      />
                      {state === "next" && ticket.blocked ? (
                        <span className="absolute -top-6 flex items-center gap-1 text-2xs font-semibold text-ink">
                          <Icon name="diamond" size={11} />
                          blocked
                        </span>
                      ) : null}
                      <button
                        {...drag}
                        type="button"
                        data-station={ticket.id}
                        disabled={ticket.missing && !onReorder}
                        onClick={() =>
                          !ticket.missing && onOpen({ id: ticket.id })
                        }
                        onKeyDown={onKey(index)}
                        aria-label={`${ticket.title}, ${columnTitle(ticket.column)}${ticket.blocked ? ", blocked" : ""}${state === "next" ? ", next stop" : ""}`}
                        className="group relative z-10 flex flex-col items-center gap-2 rounded-control px-1 pb-1 text-center disabled:cursor-default"
                      >
                        <Station state={state} colour={colour} />
                        <span
                          className={`line-clamp-2 text-xs leading-snug font-medium ${state === "served" ? "text-ink-muted" : "text-ink"} group-enabled:group-hover:underline`}
                        >
                          {ticket.title}
                        </span>
                        <span className="max-w-full truncate text-2xs font-semibold tracking-[0.02em] tabular-nums text-ink-muted">
                          {ticket.id}
                        </span>
                        <span className="text-2xs text-ink-muted">
                          {state === "missing"
                            ? "Does not exist"
                            : columnTitle(ticket.column)}
                        </span>
                      </button>
                    </>
                  )}
                </SortableStation>
              );
            })}
          </ol>
          {more ? (
            <div
              aria-hidden="true"
              className="pointer-events-none absolute inset-y-0 right-0 z-20 flex w-24 items-start justify-end bg-linear-to-l from-ground to-transparent"
            >
              {/* The hint sits in the lane above the track, clear of the stations. */}
              <span className="mt-0.5 flex items-center gap-1 rounded-control bg-ground px-1 text-2xs text-ink-muted">
                more
                <Icon name="next" size={11} />
              </span>
            </div>
          ) : null}
        </div>
      </SortableContext>
    </DndContext>
  );
}

// SortableStation is one station that can be dragged along its line.
function SortableStation({
  id,
  enabled,
  children,
}: {
  id: string;
  enabled: boolean;
  children(drag: Record<string, unknown>): ReactNode;
}) {
  const {
    setNodeRef,
    listeners,
    attributes,
    transform,
    transition,
    isDragging,
  } = useSortable({ id, disabled: !enabled });
  return (
    <li
      ref={setNodeRef}
      style={{ transform: CSS.Translate.toString(transform), transition }}
      className={`relative flex w-44 shrink-0 flex-col items-center px-2 ${isDragging ? "z-30 opacity-80" : ""}`}
    >
      {children(
        enabled
          ? { ...listeners, "aria-describedby": attributes["aria-describedby"] }
          : {},
      )}
    </li>
  );
}

function Station({
  state,
  colour,
}: {
  state: ReturnType<typeof stationState>;
  colour: string;
}) {
  switch (state) {
    case "served":
      return (
        <span
          className="grid size-7 place-items-center rounded-full"
          style={{
            background: colour,
            boxShadow: "inset 0 0 0 1px var(--fh-casing)",
          }}
        >
          <span className="size-2.5 rounded-full bg-ground" />
        </span>
      );
    case "next":
      return (
        <span className="size-7 rounded-full border-[3.5px] border-rule-strong bg-ground shadow-[0_0_0_3px_var(--fh-ground)]" />
      );
    case "missing":
      return (
        <span className="size-7 rounded-full border-2 border-dashed border-ink-faint bg-ground" />
      );
    default:
      return (
        <span
          className="size-7 rounded-full border-4 bg-ground"
          style={{ borderColor: colour }}
        />
      );
  }
}
