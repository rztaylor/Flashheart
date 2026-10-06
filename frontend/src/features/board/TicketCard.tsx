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
import { Pill, Tag } from "../../components/Pill";
import { RunStateLabel, RunStateMark } from "../../components/RunState";
import { StateNote } from "../../components/StateNote";
import type { Line } from "../../model/lines";
import { type Paint, paintVars } from "../../model/paint";
import { agentName, liveReason, STATE_LABEL } from "../../model/runs";
import { priorityLabel } from "../../model/status";
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

// TicketCard is one ticket on the board (VIEW-6), in the one anatomy of
// docs/dev/specs/ui-layout.md §2: header (id, project, running time), title,
// tags, state, live run, then by density the excerpt and next step, and a
// footer with criteria and priority. The workstream's line runs down its
// left edge; the header takes the tint of the board's "Colour by" value,
// whose tag is filled and named. Compact: id, title, type and priority.
// Normal adds the blocker, live run and criteria. Detailed adds the
// excerpt, handoff "next", attachments and review. Real blockers are pills;
// waits on an earlier station of the line are quiet.
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
    const paintedAs = (kind: string) =>
      painted?.token.startsWith(`${kind}-`) ? painted : undefined;
    const inset = line ? "pl-4" : "pl-3";
    const priority = priorityLabel(card.priority);
    const compact = density === "compact";
    const detailed = density === "detailed";
    const showCriteria = !compact && card.criteria.total > 0;
    const showFiles = detailed && (card.attachments > 0 || card.hasReview);
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
        aria-label={`${card.title}, ${card.id}${card.blocked ? ", blocked" : ""}${repair ? ", needs repair" : ""}${card.live ? `, ${liveLabel(card.live)}` : ""}${card.openQuestions > 0 && card.live?.state !== "needs-you" ? `, needs you: ${questionsWaiting(card.openQuestions).toLowerCase()}` : ""}${mirrored ? `, also in ${columnName(card.column)}` : ""}`}
        data-paint={painted?.token}
        style={paintVars(painted) as CSSProperties | undefined}
        className={`group relative flex w-full shrink-0 flex-col overflow-hidden rounded-card border bg-card text-left transition-[border-color,opacity,box-shadow,translate] duration-200 ease-out-expo hover:-translate-y-px motion-reduce:transition-none motion-reduce:hover:translate-y-0 ${
          selected
            ? "border-select shadow-[0_0_0_1px_var(--fh-select),var(--fh-shadow-card-hover)]"
            : repair
              ? "border-dashed border-ink-muted shadow-card"
              : "border-rule shadow-card hover:shadow-card-hover"
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

        {/* Rows 1 and 2: the header (id, project, running time) and the
            title, tinted with the Colour by paint. */}
        <span
          className={`flex flex-col gap-1 pr-3 ${inset} ${repair ? "pt-3.5" : "pt-2.5"} pb-2 ${
            painted
              ? done
                ? "bg-[color-mix(in_srgb,var(--paint-tint)_55%,var(--fh-card))]"
                : "bg-(--paint-tint)"
              : ""
          }`}
        >
          <span className="flex items-center gap-2 text-2xs text-ink-muted">
            <span className="shrink-0 font-semibold tracking-[0.02em] tabular-nums text-ink">
              {card.id}
            </span>
            {showProject ? (
              <span className="truncate font-medium">{showProject}</span>
            ) : null}
            {age ? (
              <span
                className="ml-auto shrink-0"
                title={`Last changed ${card.modified}`}
              >
                {age}
              </span>
            ) : null}
          </span>
          <span
            className={`line-clamp-4 text-md leading-snug heading-cut ${done ? "text-ink-muted" : "text-ink"}`}
          >
            {card.title}
          </span>
        </span>

        <span className={`flex flex-col gap-2 pr-3 pb-2.5 ${inset} pt-1`}>
          {/* Row 3: workstream, type (and age) tags, mirror note; Compact
              also carries the short blocked pill and the run mark here. */}
          <span className="flex flex-wrap items-center gap-x-1.5 gap-y-1 text-2xs text-ink-muted">
            <LineBullet line={line} size="sm" label={workstreamTitle} />
            {card.type ? (
              <Tag paint={paintedAs("type")}>{card.type}</Tag>
            ) : null}
            {paintedAs("age") ? (
              <Tag paint={paintedAs("age")}>{paintedAs("age")?.label}</Tag>
            ) : null}
            {compact && blocker ? <Pill tone="blocked">Blocked</Pill> : null}
            {compact && card.live ? (
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
          </span>

          {repair ? (
            <StateNote kind="repair" compact>
              {card.needsRepair[0] ?? "Needs repair"}
            </StateNote>
          ) : null}

          {!compact && blocker ? (
            <StateNote kind="blocked" compact>
              {more > 0 ? `${blocker.text} (+${more} more)` : blocker.text}
            </StateNote>
          ) : null}
          {!compact && !blocker && wait ? (
            <StateNote kind="waiting" compact>
              {wait.text}
            </StateNote>
          ) : null}

          {!compact && card.live ? (
            <LiveBadge live={card.live} now={now} />
          ) : null}
          {/* Another session's question about the ticket needs you even
              while its own run is not waiting on you. */}
          {!compact &&
          card.openQuestions > 0 &&
          card.live?.state !== "needs-you" ? (
            <span className="flex min-w-0 items-center gap-2 text-2xs">
              <RunStateLabel state="needs-you" />
              <span className="font-semibold text-ink">
                {questionsWaiting(card.openQuestions)}
              </span>
            </span>
          ) : null}

          {detailed && card.excerpt ? (
            <span className="line-clamp-3 text-xs text-ink-muted">
              {card.excerpt}
            </span>
          ) : null}

          {detailed && card.handoffNext ? (
            <span className="flex items-start gap-1.5 text-xs text-ink">
              <Icon name="next" size={12} className="mt-[0.2em]" />
              <span className="line-clamp-2">
                <span className="sr-only">Next: </span>
                {card.handoffNext}
              </span>
            </span>
          ) : null}

          {/* Row 10: criteria, files and review left; priority right. */}
          {showCriteria || showFiles || priority ? (
            <span
              data-row="footer"
              className="flex items-center gap-3 text-2xs text-ink-muted"
            >
              {showCriteria ? (
                <span
                  className="flex items-center gap-1"
                  title="Acceptance criteria ticked"
                >
                  <Icon name="criteria" size={13} />
                  {card.criteria.done}/{card.criteria.total}
                </span>
              ) : null}
              {detailed && card.attachments > 0 ? (
                <span className="flex items-center gap-1" title="Attachments">
                  <Icon name="attachment" size={13} />
                  {card.attachments}
                </span>
              ) : null}
              {detailed && card.hasReview ? (
                <span className="flex items-center gap-1">
                  <Icon name="review" size={13} />
                  Review
                </span>
              ) : null}
              {priority ? (
                <span className="ml-auto">
                  <Tag
                    paint={paintedAs("priority")}
                    strong={card.priority === "high"}
                  >
                    {priority}
                  </Tag>
                </span>
              ) : null}
            </span>
          ) : null}
        </span>
      </button>
    );
  },
);

// questionsWaiting says that agents' questions about a ticket wait for
// the user, when no live run of its own explains it (CARD-6).
const questionsWaiting = (count: number) =>
  count === 1 ? "Question waiting" : `${count} questions waiting`;

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
  // Line 1 is the state, the agent and the last activity; when it needs
  // you, line 2 says what it needs in words; the last line is its plan step.
  return (
    <span
      className="flex min-w-0 flex-col gap-1 text-2xs"
      data-live={live.state}
    >
      <span className="flex min-w-0 items-center gap-2">
        <RunStateLabel state={live.state} />
        {agent}
        <span
          className="ml-auto shrink-0 text-ink-muted"
          title={`Last activity ${absoluteTime(live.lastActivity)}`}
        >
          {runningTime(live.lastActivity, now)}
        </span>
      </span>
      {needsYou ? (
        <span className="font-semibold text-ink">{liveReason(live)}</span>
      ) : null}
      {step ? (
        <span className="truncate text-ink" title={live.step}>
          {step}
        </span>
      ) : null}
    </span>
  );
}
