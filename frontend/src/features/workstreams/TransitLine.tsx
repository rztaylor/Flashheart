import {
  DndContext,
  type DragEndEvent,
  PointerSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  rectSortingStrategy,
  SortableContext,
  useSortable,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import {
  type KeyboardEvent,
  type ReactNode,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import type { TicketRef, Workstream, WorkstreamTicket } from "../../api/board";
import { Icon } from "../../components/Icon";
import { Pill, StatusPill } from "../../components/Pill";
import type { Line } from "../../model/lines";
import { statusOf } from "../../model/status";
import { type Edge, layoutGraph, moveFree } from "./graph";

interface TransitLineProps {
  workstream: Workstream;
  line?: Line;
  onOpen(ticket: TicketRef): void;
  // onReorder saves a new ticket order (EDIT-4); absent when read-only. Only
  // independent stations move: the graph's shape comes from depends-on.
  onReorder?(ids: string[]): void;
}

// Graph geometry in pixels: a layer is a station's width (w-44), a lane is
// one station tall with room above the mark for its pill.
const LAYER = 176;
const LANE = 172;
const MARK = 46;
const BEND = 12;
// Suspended track dashes (trackPieces keeps overlapping ones in step).
const DASH = "10 6";

const served = (ticket: WorkstreamTicket) =>
  ticket.column === "review" ||
  ticket.column === "done" ||
  ticket.column === "archived";

// A check means finished (Done or Archived); Ready to review is served but
// never checked (ui-layout.md §4). An unfinished ticket that can start is a
// next stop; one held by something outside the line is a dashed ring.
function stationState(ticket: WorkstreamTicket, suspended: boolean) {
  if (ticket.missing) return "missing";
  if (ticket.column === "done" || ticket.column === "archived") return "done";
  if (served(ticket)) return "served";
  if (ticket.held || suspended) return "held";
  if (!ticket.blocked) return "next";
  return "ahead";
}
type State = ReturnType<typeof stationState>;

const columnTitle = (column: WorkstreamTicket["column"]) =>
  statusOf(column).label;

type Track = "served" | "ahead" | "suspended";

// TransitLine draws a workstream as a railway graph (VIEW-4, ui-layout.md
// §4). Tickets are stations, each with its title, id and status pill; a
// depends-on link between two of them is track. A chain runs along one
// line, independent work runs on parallel tracks, and a ticket that depends
// on several others is where their tracks join. Track into a served station
// is solid line colour, track ahead is lighter, and only suspended service
// is dashed (the line's own workstream dependencies are unmet, or the
// station is held from outside the line). Tickets that can start are the
// larger interchange rings. Tickets with no links on the line wait below as
// independent stations. Stations sit on the route card's surface
// (--route-surface).
export function TransitLine({
  workstream,
  line,
  onOpen,
  onReorder,
}: TransitLineProps) {
  const colour = line ? `var(--fh-line-${line.colour})` : "var(--fh-ink-faint)";
  const layout = useMemo(
    () => layoutGraph(workstream.tickets),
    [workstream.tickets],
  );
  const byID = useMemo(() => {
    const map = new Map<string, WorkstreamTicket>();
    for (const ticket of workstream.tickets)
      if (!map.has(ticket.id)) map.set(ticket.id, ticket);
    return map;
  }, [workstream.tickets]);

  if (workstream.tickets.length === 0) {
    return (
      <p className="py-4 text-xs text-ink-faint">
        No tickets on this line yet.
      </p>
    );
  }

  const station = (
    ticket: WorkstreamTicket,
    drag?: Record<string, unknown>,
  ) => (
    <StationButton
      ticket={ticket}
      state={stationState(ticket, workstream.suspended)}
      colour={colour}
      titles={byID}
      onOpen={onOpen}
      drag={drag}
    />
  );
  const free = layout.free.flatMap((id) => byID.get(id) ?? []);
  return (
    <div className="flex flex-col gap-2">
      {layout.stations.length > 0 ? (
        <Graph
          workstream={workstream}
          layout={layout}
          byID={byID}
          colour={colour}
          station={station}
        />
      ) : null}
      {free.length > 0 ? (
        <FreeStations
          workstream={workstream}
          tickets={free}
          captioned={layout.stations.length > 0}
          station={station}
          onReorder={onReorder}
        />
      ) : null}
    </div>
  );
}

// Graph lays the linked stations out on their tracks, scrolling sideways
// with an edge fade and a "more" hint when wider than the card.
function Graph({
  workstream,
  layout,
  byID,
  colour,
  station,
}: {
  workstream: Workstream;
  layout: ReturnType<typeof layoutGraph>;
  byID: Map<string, WorkstreamTicket>;
  colour: string;
  station(ticket: WorkstreamTicket): ReactNode;
}) {
  const scroller = useRef<HTMLDivElement>(null);
  const [more, setMore] = useState(false);
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

  const width = layout.layers * LAYER;
  const height = layout.lanes * LANE;
  const position = new Map(layout.stations.map((item) => [item.id, item]));
  const trackInto = (ticket: WorkstreamTicket | undefined): Track => {
    if (!ticket) return "ahead";
    if (served(ticket)) return "served";
    if (workstream.suspended || ticket.held || ticket.missing)
      return "suspended";
    return "ahead";
  };
  // Reading order: by layer, then track, so Tab follows the line.
  const ordered = [...layout.stations].sort(
    (a, b) => a.layer - b.layer || a.lane - b.lane,
  );
  return (
    <div className="relative">
      <div ref={scroller} className="overflow-x-auto pt-1 pb-2">
        <div className="relative" style={{ width, height }}>
          <svg
            aria-hidden="true"
            className="absolute inset-0"
            width={width}
            height={height}
            fill="none"
          >
            {layout.edges.map((edge) => {
              const from = position.get(edge.from);
              const to = position.get(edge.to);
              if (!from || !to) return null;
              const track = trackInto(byID.get(edge.to));
              return (
                <g
                  key={`${edge.from}>${edge.to}`}
                  data-track={track}
                  stroke={colour}
                  strokeWidth={6}
                  strokeOpacity={track === "ahead" ? 0.35 : 1}
                >
                  {trackPieces(edge, from, to).map((piece) => (
                    <path
                      key={piece.d}
                      d={piece.d}
                      strokeDasharray={track === "suspended" ? DASH : undefined}
                      strokeDashoffset={
                        track === "suspended" ? piece.offset : undefined
                      }
                    />
                  ))}
                </g>
              );
            })}
          </svg>
          <ol aria-label={`${workstream.title} stations`}>
            {ordered.map((item) => {
              const ticket = byID.get(item.id);
              if (!ticket) return null;
              return (
                <li
                  key={item.id}
                  className="absolute flex w-44 flex-col items-center px-2"
                  style={{ left: item.layer * LAYER, top: item.lane * LANE }}
                >
                  {station(ticket)}
                </li>
              );
            })}
          </ol>
        </div>
      </div>
      {more ? (
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-y-0 right-0 z-20 flex w-24 items-start justify-end bg-linear-to-l from-(--route-surface) to-transparent"
        >
          <span className="mt-0.5 flex items-center gap-1 rounded-control bg-(--route-surface) px-1 text-2xs text-ink-muted">
            more
            <Icon name="next" size={11} />
          </span>
        </div>
      ) : null}
    </div>
  );
}

// trackPieces draws an edge between two stations' marks: straight along a
// track, or with rounded right-angle bends in the gap between columns. Each
// straight piece runs left to right or top to bottom with a dash offset
// fixed by where it starts, so suspended pieces that overlap (two edges
// sharing a stretch of track) keep their dashes in step.
export function trackPieces(
  edge: Edge,
  from: { layer: number; lane: number },
  to: { layer: number; lane: number },
) {
  const x = (layer: number) => layer * LAYER + LAYER / 2;
  const y = (lane: number) => lane * LANE + MARK;
  const [x1, y1, x2, y2] = [
    x(from.layer),
    y(from.lane),
    x(to.layer),
    y(to.lane),
  ];
  const pieces: { d: string; offset: number }[] = [];
  const along = (start: number) => (((start - x(0)) % 16) + 16) % 16;
  const h = (xa: number, xb: number, at: number) => {
    if (xa === xb) return;
    const [a, b] = [Math.min(xa, xb), Math.max(xa, xb)];
    pieces.push({ d: `M ${a} ${at} H ${b}`, offset: along(a) });
  };
  const v = (at: number, ya: number, yb: number) => {
    if (ya === yb) return;
    const [a, b] = [Math.min(ya, yb), Math.max(ya, yb)];
    pieces.push({ d: `M ${at} ${a} V ${b}`, offset: ((a % 16) + 16) % 16 });
  };
  // turn bends from track ya to track yb at bx, leaving ends at bx ± BEND.
  const turn = (bx: number, ya: number, yb: number) => {
    const s = Math.sign(yb - ya);
    pieces.push({
      d: `M ${bx - BEND} ${ya} Q ${bx} ${ya} ${bx} ${ya + s * BEND}`,
      offset: 0,
    });
    v(bx, ya + s * BEND, yb - s * BEND);
    pieces.push({
      d: `M ${bx} ${yb - s * BEND} Q ${bx} ${yb} ${bx + BEND} ${yb}`,
      offset: 0,
    });
  };
  const after = x1 + LAYER / 2;
  const before = x2 - LAYER / 2;
  switch (edge.route) {
    case "straight":
      h(x1, x2, y1);
      break;
    case "branch":
    case "merge": {
      const bx = edge.route === "branch" ? after : before;
      h(x1, bx - BEND, y1);
      turn(bx, y1, y2);
      h(bx + BEND, x2, y2);
      break;
    }
    case "detour": {
      const yv = y(edge.via ?? 0);
      h(x1, after - BEND, y1);
      turn(after, y1, yv);
      h(after + BEND, before - BEND, yv);
      turn(before, yv, y2);
      h(before + BEND, x2, y2);
      break;
    }
  }
  return pieces;
}

// FreeStations are the tickets with no links on the line, wrapped in rows;
// their order is display order and can be changed (EDIT-4).
function FreeStations({
  workstream,
  tickets,
  captioned,
  station,
  onReorder,
}: {
  workstream: Workstream;
  tickets: WorkstreamTicket[];
  captioned: boolean;
  station(ticket: WorkstreamTicket, drag?: Record<string, unknown>): ReactNode;
  onReorder?(ids: string[]): void;
}) {
  const list = useRef<HTMLUListElement>(null);
  const refocus = useRef("");
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
  );
  const ids = tickets.map((ticket) => ticket.id);

  // A station moved with the keyboard keeps focus at its new place.
  // biome-ignore lint/correctness/useExhaustiveDependencies: runs after the order changes.
  useEffect(() => {
    if (!refocus.current) return;
    list.current
      ?.querySelector<HTMLButtonElement>(`[data-station="${refocus.current}"]`)
      ?.focus();
    refocus.current = "";
  }, [workstream.tickets]);

  const save = (from: number, to: number) =>
    onReorder?.(
      moveFree(
        workstream.tickets.map((ticket) => ticket.id),
        ids,
        from,
        to,
      ),
    );
  const onKey = (index: number) => (event: KeyboardEvent<HTMLElement>) => {
    if (!onReorder || !event.shiftKey) return;
    const step =
      event.key === "ArrowLeft" ? -1 : event.key === "ArrowRight" ? 1 : 0;
    const to = index + step;
    if (step === 0 || to < 0 || to >= ids.length) return;
    event.preventDefault();
    refocus.current = ids[index] ?? "";
    save(index, to);
  };
  const onDragEnd = (event: DragEndEvent) => {
    const from = ids.indexOf(String(event.active.id));
    const to = event.over ? ids.indexOf(String(event.over.id)) : -1;
    if (from >= 0 && to >= 0 && from !== to) save(from, to);
  };

  return (
    <div>
      {captioned ? (
        <p className="pt-1 text-2xs font-semibold tracking-[0.02em] text-ink-muted">
          No dependencies on this line
        </p>
      ) : null}
      <DndContext
        sensors={sensors}
        onDragEnd={onDragEnd}
        accessibility={{
          screenReaderInstructions: {
            draggable:
              "Press Shift with the left or right arrow to move this station earlier or later, or drag it with the pointer.",
          },
        }}
      >
        <SortableContext
          items={ids}
          strategy={rectSortingStrategy}
          disabled={!onReorder}
        >
          <ul
            ref={list}
            aria-label={`${workstream.title} stations with no dependencies on the line`}
            className="flex flex-wrap gap-y-4 pt-1 pb-2"
          >
            {tickets.map((ticket, index) => (
              <SortableStation
                key={ticket.id}
                id={ticket.id}
                enabled={!!onReorder}
                onKeyDown={onKey(index)}
              >
                {(drag) => station(ticket, drag)}
              </SortableStation>
            ))}
          </ul>
        </SortableContext>
      </DndContext>
    </div>
  );
}

// SortableStation is one independent station that can be dragged.
function SortableStation({
  id,
  enabled,
  onKeyDown,
  children,
}: {
  id: string;
  enabled: boolean;
  onKeyDown(event: KeyboardEvent<HTMLElement>): void;
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
      onKeyDown={onKeyDown}
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

// StationButton is one station: its mark, title, id and status pill, with a
// pill above naming an unfinished dependency outside the line.
function StationButton({
  ticket,
  state,
  colour,
  titles,
  onOpen,
  drag,
}: {
  ticket: WorkstreamTicket;
  state: State;
  colour: string;
  titles: Map<string, WorkstreamTicket>;
  onOpen(ticket: TicketRef): void;
  drag?: Record<string, unknown>;
}) {
  const needs = [...ticket.dependsOn, ...ticket.outside];
  const label = [
    ticket.title,
    columnTitle(ticket.column),
    ticket.blocked ? "blocked" : "",
    state === "next" ? "next stop" : "",
    needs.length > 0
      ? `needs ${needs.map((id) => titles.get(id)?.title ?? id).join(" and ")}`
      : "",
  ]
    .filter(Boolean)
    .join(", ");
  return (
    <>
      <span className="flex h-7 items-start">
        {ticket.outside.length > 0 ? (
          <Pill tone="blocked">
            {`Needs ${ticket.outside[0]}${ticket.outside.length > 1 ? ` +${ticket.outside.length - 1}` : ""}`}
          </Pill>
        ) : null}
      </span>
      <button
        {...drag}
        type="button"
        data-station={ticket.id}
        data-station-state={state}
        disabled={ticket.missing}
        onClick={() => !ticket.missing && onOpen({ id: ticket.id })}
        aria-label={label}
        className="group relative z-10 flex flex-col items-center gap-1.5 rounded-control px-1 pb-1 text-center disabled:cursor-default"
      >
        <span className="grid h-9 place-items-center">
          <Mark state={state} colour={colour} />
        </span>
        <span className="line-clamp-2 text-sm leading-snug font-semibold text-ink group-enabled:group-hover:underline">
          {ticket.title}
        </span>
        <span className="max-w-full truncate text-2xs font-semibold tracking-[0.02em] tabular-nums text-ink-muted">
          {ticket.id}
        </span>
        {state === "missing" ? (
          <span className="text-2xs text-ink-muted">Does not exist</span>
        ) : (
          <StatusPill column={ticket.column} />
        )}
      </button>
    </>
  );
}

function Mark({ state, colour }: { state: State; colour: string }) {
  switch (state) {
    case "done":
      return (
        <span
          className="grid size-8 place-items-center rounded-full"
          style={{
            background: colour,
            color: "var(--route-ink)",
            boxShadow:
              "inset 0 0 0 1px var(--fh-casing), 0 0 0 3px var(--route-surface)",
          }}
        >
          <Icon name="check" size={16} className="[stroke-width:3]" />
        </span>
      );
    case "served":
      return (
        <span
          className="grid size-8 place-items-center rounded-full"
          style={{
            background: colour,
            boxShadow:
              "inset 0 0 0 1px var(--fh-casing), 0 0 0 3px var(--route-surface)",
          }}
        >
          <span className="size-2.5 rounded-full bg-(--route-surface)" />
        </span>
      );
    case "next":
      return (
        <span
          className="grid size-9 place-items-center rounded-full border-[5px] bg-(--route-surface) shadow-[0_0_0_4px_var(--route-surface)]"
          style={{ borderColor: colour }}
        >
          <span
            className="size-2.5 rounded-full"
            style={{ background: colour }}
          />
        </span>
      );
    case "held":
      return (
        <span
          className="size-7 rounded-full border-4 border-dashed bg-(--route-surface) shadow-[0_0_0_3px_var(--route-surface)]"
          style={{ borderColor: colour }}
        />
      );
    case "missing":
      return (
        <span className="size-7 rounded-full border-2 border-dashed border-ink-faint bg-(--route-surface)" />
      );
    default:
      return (
        <span
          className="size-7 rounded-full border-4 bg-(--route-surface) shadow-[0_0_0_3px_var(--route-surface)]"
          style={{ borderColor: colour }}
        />
      );
  }
}
