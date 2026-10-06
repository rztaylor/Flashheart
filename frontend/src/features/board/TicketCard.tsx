import {
  type CSSProperties,
  forwardRef,
  type HTMLAttributes,
  type KeyboardEvent,
} from "react";

import { type Card, COLUMNS, splitReasons } from "../../api/board";
import type { Live } from "../../api/runs";
import { Icon } from "../../components/Icon";
import { LineBullet } from "../../components/LineBullet";
import { RunStateLabel, RunStateMark } from "../../components/RunState";
import { StateNote } from "../../components/StateNote";
import type { Line } from "../../model/lines";
import { type Paint, paintVars } from "../../model/paint";
import { agentName, liveReason, STATE_LABEL } from "../../model/runs";
import { absoluteTime, runningTime } from "../../model/time";
import type { Density } from "../filters/FilterBar";

interface TicketCardProps {
  card: Card;
  line?: Line;
  workstreamTitle?: string;
  density: Density;
  paint?: Paint;
  dimmed?: boolean;
  showProject?: string;
  selected?: boolean;
  tabIndex: number;
  now: Date;
  onOpen(): void;
  onKeyDown?(event: KeyboardEvent<HTMLButtonElement>): void;
  onFocus?(): void;
  // dragProps carries the drag source's pointer listeners and description.
  dragProps?: HTMLAttributes<HTMLButtonElement>;
  // lifted draws the card being dragged; ghost the place it left.
  lifted?: boolean;
  ghost?: boolean;
  // mirrored marks a copy shown in a virtual column (VIEW-2); the ticket
  // lives in its real column.
  mirrored?: boolean;
}

const priorityLabel: Record<string, string> = {
  high: "High",
  medium: "Medium",
  low: "Low",
};

// TicketCard is one ticket on the board (VIEW-6). The workstream's line runs
// down its left edge; the header takes a tint of the board's "Colour by"
// attribute, named in a tag of the strong shade. Compact: title, id, type and
// priority. Normal adds the blocked proof and criteria. Detailed adds the
// excerpt, handoff "next" and attachments. Real blockers are marked; waits on
// an earlier station of the line are quiet.
export const TicketCard = forwardRef<HTMLButtonElement, TicketCardProps>(
  function TicketCard(
    {
      card,
      line,
      workstreamTitle,
      density,
      paint,
      dimmed,
      showProject,
      selected,
      tabIndex,
      now,
      onOpen,
      onKeyDown,
      onFocus,
      dragProps,
      lifted,
      ghost,
      mirrored,
    },
    ref,
  ) {
    const repair = card.needsRepair.length > 0;
    const done = card.column === "done";
    const age = runningTime(card.modified, now);
    const { blockers, waits } = splitReasons(card.blockedBy);
    const blocker = blockers[0];
    const wait = waits[0];
    const more = card.blockedBy.length - 1;
    const painted = paint && !repair ? paint : undefined;
    const inset = line ? "pl-4" : "pl-3";
    const priority = card.priority
      ? (priorityLabel[card.priority] ?? card.priority)
      : "";
    return (
      <button
        {...dragProps}
        ref={ref}
        type="button"
        tabIndex={tabIndex}
        onClick={onOpen}
        onKeyDown={onKeyDown}
        onFocus={onFocus}
        // Only the real card is the current ticket; its mirror shares the
        // selection ring but not the announcement.
        aria-current={selected && !mirrored ? "true" : undefined}
        data-ticket={card.id}
        data-mirrored={mirrored ? "" : undefined}
        aria-label={`${card.title}, ${card.id}${card.blocked ? ", blocked" : ""}${repair ? ", needs repair" : ""}${card.live ? `, ${liveLabel(card.live)}` : ""}${mirrored ? `, also in ${columnName(card.column)}` : ""}`}
        data-paint={painted?.token}
        style={paintVars(painted) as CSSProperties | undefined}
        className={`group relative flex w-full shrink-0 flex-col overflow-hidden rounded-card border bg-card text-left transition-[border-color,opacity,box-shadow,translate] duration-200 ease-out-expo hover:-translate-y-px ${
          selected
            ? "border-rule-strong shadow-[0_0_0_1px_var(--fh-rule-strong),var(--fh-shadow-card-hover)]"
            : repair
              ? "border-dashed border-ink-muted shadow-card"
              : "border-rule/70 shadow-card hover:shadow-card-hover"
        } ${dimmed ? "opacity-35" : ""} ${ghost ? "opacity-30" : ""} ${lifted ? "rotate-[1.2deg] cursor-grabbing shadow-card-hover" : ""}`}
      >
        {line ? (
          <span
            aria-hidden="true"
            className="absolute inset-y-0 left-0 w-1"
            style={{ background: `var(--fh-line-${line.colour})` }}
          />
        ) : null}
        {repair ? (
          <span
            aria-hidden="true"
            className="hatched-strong absolute inset-x-0 top-0 h-1.5"
          />
        ) : null}
        <span
          className={`flex items-start gap-2 pr-3 ${inset} ${repair ? "pt-3.5" : "pt-2.5"} ${
            painted
              ? `pb-2 ${done ? "bg-[color-mix(in_srgb,var(--paint-tint)_55%,var(--fh-card))]" : "bg-(--paint-tint)"}`
              : "pb-1.5"
          }`}
        >
          <LineBullet line={line} size="sm" label={workstreamTitle} />
          <span
            className={`min-w-0 flex-1 text-sm leading-snug font-medium ${done ? "text-ink-muted" : "text-ink"}`}
          >
            {card.title}
          </span>
          {age ? (
            <span
              className="shrink-0 text-2xs text-ink-muted"
              title={`Last changed ${card.modified}`}
            >
              {age}
            </span>
          ) : null}
        </span>

        <span
          className={`flex flex-col gap-1.5 pr-3 pb-2.5 ${inset} ${painted ? "pt-2" : ""}`}
        >
          <span className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-2xs text-ink-muted">
            {showProject ? (
              <span className="font-medium text-ink">{showProject}</span>
            ) : null}
            <span className="shrink-0 font-semibold tracking-[0.02em] tabular-nums text-ink">
              {card.id}
            </span>
            {painted ? (
              <span className="rounded-[3px] bg-(--paint) px-1.5 leading-4 font-semibold text-(--paint-ink)">
                {painted.label}
              </span>
            ) : null}
            {card.type && painted?.token.startsWith("type-") !== true ? (
              <span>{card.type}</span>
            ) : null}
            {priority && painted?.token.startsWith("priority-") !== true ? (
              <span
                className={
                  card.priority === "high"
                    ? "font-semibold text-ink"
                    : undefined
                }
              >
                {priority}
              </span>
            ) : null}
            {density === "compact" && card.live ? (
              card.live.state === "needs-you" ? (
                <RunStateLabel state="needs-you" />
              ) : (
                <span
                  title={`${agentName(card.live.agent)}: ${STATE_LABEL[card.live.state]}`}
                >
                  <RunStateMark state={card.live.state} size={10} />
                </span>
              )
            ) : null}
            {mirrored ? (
              <span className="text-ink-faint">
                in {columnName(card.column)}
              </span>
            ) : null}
            {density === "compact" && blocker ? (
              <span className="flex items-center gap-0.5 font-semibold text-ink">
                <Icon name="diamond" size={10} />
                Blocked
              </span>
            ) : null}
          </span>

          {density !== "compact" && card.live ? (
            <LiveBadge live={card.live} now={now} />
          ) : null}

          {repair ? (
            <StateNote kind="repair" compact>
              {card.needsRepair[0] ?? "Needs repair"}
            </StateNote>
          ) : null}

          {density !== "compact" && blocker ? (
            <StateNote kind="blocked" compact>
              {more > 0 ? `${blocker.text} (+${more} more)` : blocker.text}
            </StateNote>
          ) : null}
          {density !== "compact" && !blocker && wait ? (
            <StateNote kind="waiting" compact>
              {wait.text}
            </StateNote>
          ) : null}

          {density === "detailed" && card.excerpt ? (
            <span className="line-clamp-3 text-xs text-ink-muted">
              {card.excerpt}
            </span>
          ) : null}

          {density === "detailed" && card.handoffNext ? (
            <span className="flex items-start gap-1.5 text-xs text-ink">
              <Icon name="next" size={12} className="mt-[0.2em]" />
              <span className="line-clamp-2">
                <span className="sr-only">Next: </span>
                {card.handoffNext}
              </span>
            </span>
          ) : null}

          {density !== "compact" &&
          (card.criteria.total > 0 ||
            (density === "detailed" &&
              (card.attachments > 0 || card.hasReview))) ? (
            <span className="flex items-center gap-3 text-2xs text-ink-muted">
              {card.criteria.total > 0 ? (
                <span
                  className="flex items-center gap-1"
                  title="Acceptance criteria ticked"
                >
                  <Icon name="criteria" size={12} />
                  {card.criteria.done}/{card.criteria.total}
                </span>
              ) : null}
              {density === "detailed" && card.attachments > 0 ? (
                <span className="flex items-center gap-1" title="Attachments">
                  <Icon name="attachment" size={12} />
                  {card.attachments}
                </span>
              ) : null}
              {density === "detailed" && card.hasReview ? (
                <span className="flex items-center gap-1">
                  <Icon name="review" size={12} />
                  Review
                </span>
              ) : null}
            </span>
          ) : null}
        </span>
      </button>
    );
  },
);

const columnName = (id: string) =>
  COLUMNS.find((column) => column.id === id)?.title ?? id;

// liveLabel is the live badge in words, for the card's accessible name.
function liveLabel(live: Live): string {
  const state =
    live.state === "needs-you"
      ? `needs you: ${liveReason(live)}`
      : STATE_LABEL[live.state].toLowerCase();
  const plan =
    live.total > 0
      ? `, plan ${live.done} of ${live.total}${live.step ? `: ${live.step}` : ""}`
      : "";
  return `${agentName(live.agent)} ${state}${plan}`;
}

// LiveBadge is the card's live run (VIEW-8): its state in ink and words, the
// agent, the plan step and the time since its last activity.
function LiveBadge({ live, now }: { live: Live; now: Date }) {
  const step =
    live.total > 0
      ? `${live.done}/${live.total}${live.step ? ` · ${live.step}` : ""}`
      : "";
  const needsYou = live.state === "needs-you";
  const agent = (
    <span className="shrink-0 text-ink-muted">{agentName(live.agent)}</span>
  );
  // Line 1 is the state and, when it needs you, what it needs in words;
  // line 2 is the agent and its plan step.
  return (
    <span
      className="flex min-w-0 flex-col gap-0.5 text-2xs"
      data-live={live.state}
    >
      <span className="flex min-w-0 items-center gap-2">
        <RunStateLabel state={live.state} />
        {needsYou ? (
          <span className="font-semibold text-ink">{liveReason(live)}</span>
        ) : (
          agent
        )}
        <span
          className="ml-auto shrink-0 text-ink-muted"
          title={`Last activity ${absoluteTime(live.lastActivity)}`}
        >
          {runningTime(live.lastActivity, now)}
        </span>
      </span>
      {needsYou || step ? (
        <span className="flex min-w-0 items-center gap-2">
          {needsYou ? agent : null}
          {step ? (
            <span className="truncate text-ink" title={live.step}>
              {step}
            </span>
          ) : null}
        </span>
      ) : null}
    </span>
  );
}
