import {
  type CSSProperties,
  forwardRef,
  type HTMLAttributes,
  type KeyboardEvent,
  type ReactNode,
} from "react";

import type { Card } from "../../api/board";
import type { Live } from "../../api/runs";
import { Icon } from "../../components/Icon";
import { NeedsYouPlate, plateClass } from "../../components/NeedsYou";
import { StatusPill } from "../../components/Pill";
import { RunStateMark } from "../../components/RunState";
import { StateNote } from "../../components/StateNote";
import { TicketLink } from "../../components/TicketLink";
import { PriorityMarker, TypeIcon } from "../../components/TicketMarks";
import type { Line } from "../../model/lines";
import { type Paint, paintVars } from "../../model/paint";
import { requestWords } from "../../model/requests";
import { agentName, needsReason, STATE_LABEL } from "../../model/runs";
import { priorityLabel } from "../../model/status";
import { absoluteTime, runningTime } from "../../model/time";
import type { Density } from "../filters/FilterBar";

interface TicketCardProps {
  card: Card;
  line?: Line;
  workstreamTitle?: string;
  density: Density;
  paint?: Paint;
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
}

// TicketCard is one ticket on the board (VIEW-6): one set of markers in
// three layouts (docs/dev/specs/ui-layout.md §2, FH-147). Compact is the
// title, led by its id, with only the attention and priority marks beside
// it. Normal is the title, one meta line of markers (type, id, blocked,
// change requests, live run, age, review, attachments, priority) and, when
// something needs the user, its reason on a line of its own. Detailed adds
// the words: the blocker's reason, the run's agent and plan step, the next
// step, an excerpt and the workstream. The workstream's line runs down the
// left edge, and Colour by tints the whole card.
export const TicketCard = forwardRef<HTMLButtonElement, TicketCardProps>(
  function TicketCard(
    {
      card,
      line,
      workstreamTitle,
      density,
      paint,
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
    },
    ref,
  ) {
    const repair = card.needsRepair.length > 0;
    const done = card.column === "done";
    // A rejected ticket sits in Done; its pill says it was not accepted.
    const rejected = done && card.rejected === true;
    const painted = paint && !repair ? paint : undefined;
    const inset = line ? "pl-4" : "pl-3";
    const compact = density === "compact";
    const detailed = density === "detailed";
    const marks = cardMarks(card, now, onOpen);
    const titleClass = `text-base leading-snug font-semibold ${done ? "text-ink-muted" : "text-ink"}`;
    const blocker = card.blockedBy[0];
    const more = card.blockedBy.length - 1;
    // The card is drawn under one button that covers it, so a click
    // anywhere opens the panel and the card stays one keyboard stop. Only
    // the id, a link to the ticket's full page, and the markers, whose
    // words are in their tooltips, sit above the button. The drawing is
    // hidden from assistive technology, which hears the button's name, and
    // the panel header carries the same link for the keyboard.
    return (
      <div
        data-paint={painted?.token}
        style={paintVars(painted) as CSSProperties | undefined}
        className={`group relative isolate flex w-full shrink-0 flex-col overflow-hidden rounded-card border text-left transition-[border-color,opacity,box-shadow,translate] duration-200 ease-out-expo hover:-translate-y-px motion-reduce:transition-none motion-reduce:hover:translate-y-0 ${
          painted
            ? done
              ? "bg-[color-mix(in_srgb,var(--paint-tint)_55%,var(--fh-card))]"
              : "bg-(--paint-tint)"
            : "bg-card"
        } ${
          selected
            ? "border-select shadow-[0_0_0_1px_var(--fh-select),var(--fh-shadow-card-hover)]"
            : repair
              ? "border-dashed border-ink-muted shadow-card"
              : "border-rule shadow-card hover:shadow-card-hover"
        } ${ghost ? "opacity-30" : ""} ${lifted ? "rotate-[1.2deg] cursor-grabbing shadow-card-hover" : ""}`}
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
          aria-hidden="true"
          className={`flex flex-col gap-1.5 pr-3 ${inset} ${repair ? "pt-3.5" : "pt-2.5"} ${compact ? "pb-2.5" : "pb-3"}`}
        >
          {compact ? (
            // Compact: the id starts the title; beside it only what needs
            // attention and the priority.
            <span className="flex items-start gap-2">
              <span
                data-part="title"
                title={card.title}
                className={`line-clamp-2 min-w-0 flex-1 ${titleClass}`}
              >
                <TicketLink
                  id={card.id}
                  tabIndex={-1}
                  className="relative z-[2] mr-1.5 text-2xs text-ink"
                />
                {card.title}
              </span>
              <span
                data-part="marks"
                className="flex h-[1.35rem] shrink-0 items-center gap-1.5"
              >
                {marks.needsYou}
                {marks.blocked}
                {marks.requests}
                {marks.priority}
              </span>
            </span>
          ) : (
            <>
              <span
                data-part="title"
                title={card.title}
                className={`line-clamp-3 ${titleClass}`}
              >
                {card.title}
              </span>

              {repair ? (
                <StateNote kind="repair" compact>
                  {card.needsRepair[0] ?? "Needs repair"}
                </StateNote>
              ) : null}

              {/* The meta line never wraps: the markers that can go (age,
                  review, attachments, last first) are last in the line, and
                  one that does not fit wraps out of sight. The rest come
                  first and fit a 15rem column. Priority holds the end. */}
              <span
                data-row="meta"
                className="flex items-center gap-2 text-2xs text-ink-muted"
              >
                <span className="flex h-4 min-w-0 flex-1 flex-wrap content-start items-center gap-x-1.5 overflow-hidden">
                  {marks.type}
                  <TicketLink
                    id={card.id}
                    tabIndex={-1}
                    className="relative z-[2] h-4 shrink-0 leading-4 text-ink"
                  />
                  {rejected ? (
                    <StatusPill
                      column={card.column}
                      rejected
                      className="h-4 leading-4"
                    />
                  ) : null}
                  {marks.blocked}
                  {marks.requests}
                  {marks.live}
                  {marks.age}
                  {marks.review}
                  {marks.attachments}
                </span>
                {marks.priority}
              </span>

              {marks.needsLine}

              {detailed && blocker ? (
                <StateNote kind="blocked" compact>
                  {more > 0 ? `${blocker.text} (+${more} more)` : blocker.text}
                </StateNote>
              ) : null}

              {detailed && card.live ? (
                <RunLine live={card.live} now={now} />
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

              {detailed && card.excerpt ? (
                <span
                  data-row="excerpt"
                  className="line-clamp-2 text-xs text-ink-muted"
                >
                  {card.excerpt}
                </span>
              ) : null}

              {detailed && (workstreamTitle || showProject) ? (
                <span
                  data-row="workstream"
                  className="truncate text-2xs font-medium text-ink-muted"
                >
                  {[showProject, workstreamTitle].filter(Boolean).join(" · ")}
                </span>
              ) : null}
            </>
          )}
        </span>
        <button
          {...dragProps}
          ref={ref}
          type="button"
          tabIndex={tabIndex}
          onClick={onOpen}
          onKeyDown={onKeyDown}
          onFocus={onFocus}
          aria-current={selected ? "true" : undefined}
          data-ticket={card.id}
          aria-label={cardName(card, now, { workstreamTitle, showProject })}
          // The card clips its edges, so the focus ring is drawn inside.
          className="absolute inset-0 z-[1] rounded-card focus-visible:-outline-offset-2"
        />
      </div>
    );
  },
);

// Mark is one marker drawn on a card, its words in its tooltip. It sits
// above the card's covering button so the tooltip shows, and a click on it
// still opens the card.
function Mark({
  mark,
  label,
  onOpen,
  className,
  children,
}: {
  mark: string;
  label: string;
  onOpen(): void;
  className?: string;
  children: ReactNode;
}) {
  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: a pointer shortcut to the card's own button, which is the control.
    // biome-ignore lint/a11y/useKeyWithClickEvents: the card's button takes the keys.
    <span
      data-mark={mark}
      title={label}
      onClick={onOpen}
      className={`relative z-[2] flex h-4 shrink-0 items-center gap-0.5 ${className ?? ""}`}
    >
      {children}
    </span>
  );
}

// cardMarks are the card's markers, each only when it applies: one set,
// laid out by each density.
function cardMarks(card: Card, now: Date, onOpen: () => void) {
  const requests = requestWords(card.changeRequests);
  const priority = priorityLabel(card.priority);
  const age = runningTime(card.modified, now);
  const live = card.live;
  const needsYou = needsWords(card);
  const mark = (
    name: string,
    label: string,
    children: ReactNode,
    className?: string,
  ) => (
    <Mark mark={name} label={label} onOpen={onOpen} className={className}>
      {children}
    </Mark>
  );
  return {
    type: card.type
      ? mark("type", card.type, <TypeIcon type={card.type} size={12} />)
      : null,
    blocked: card.blocked
      ? mark(
          "blocked",
          blockedWords(card),
          <Icon
            name="blocked"
            size={12}
            className="text-(--fh-state-blocked-ink)"
          />,
        )
      : null,
    requests:
      requests && card.changeRequests
        ? mark(
            "requests",
            requests,
            <>
              <Icon name="comment" size={12} />
              <span className="font-semibold">
                {card.changeRequests.open || card.changeRequests.addressed}
              </span>
            </>,
            card.changeRequests.open
              ? "text-(--fh-state-progress-ink)"
              : "text-(--fh-state-review-ink)",
          )
        : null,
    live: live
      ? mark(
          "live",
          `${agentName(live.agent)}: ${STATE_LABEL[live.state]}${
            live.total > 0
              ? ` · plan ${live.done} of ${live.total}${live.step ? `: ${live.step}` : ""}`
              : ""
          }`,
          <>
            <RunStateMark state={live.state} size={11} />
            {live.total > 0 ? (
              <span className="font-medium text-ink">
                {live.done}/{live.total}
              </span>
            ) : null}
          </>,
          live.state === "working" || live.state === "needs-you"
            ? "text-ink"
            : undefined,
        )
      : null,
    age: age
      ? mark("age", `Last changed ${absoluteTime(card.modified)}`, age)
      : null,
    review: card.hasReview
      ? mark("review", "Review written", <Icon name="review" size={12} />)
      : null,
    attachments:
      card.attachments > 0
        ? mark(
            "attachments",
            attachmentWords(card.attachments),
            <>
              <Icon name="attachment" size={12} />
              {card.attachments}
            </>,
          )
        : null,
    priority: priority
      ? mark(
          "priority",
          `${priority} priority`,
          <PriorityMarker priority={card.priority} />,
          "ml-auto",
        )
      : null,
    // needsYou is the attention plate beside a Compact title.
    needsYou: needsYou
      ? mark(
          "needs-you",
          `Needs you: ${needsYou}`,
          <span className={`${plateClass("inline")} px-1.5`}>
            <RunStateMark state="needs-you" size={9} />
          </span>,
        )
      : null,
    // needsLine is Normal's and Detailed's: the plate and why, in words.
    needsLine: needsYou ? (
      <span
        data-row="needs-you"
        className="flex min-w-0 items-start gap-2 text-2xs"
      >
        <NeedsYouPlate className="shrink-0" />
        <span className="line-clamp-2 min-w-0 pt-0.5 font-semibold text-ink">
          {needsYou}
        </span>
      </span>
    ) : null,
  };
}

// needsWords says why the card needs the user: its run's reason, or the
// questions about it waiting when its run is not waiting on the user
// (CARD-6); "" when nothing needs the user.
function needsWords(card: Card): string {
  if (card.live?.state === "needs-you")
    return needsReason(card.live.reason) || "Needs you";
  return card.openQuestions > 0 ? questionsWaiting(card.openQuestions) : "";
}

function blockedWords(card: Card): string {
  const first = card.blockedBy[0];
  if (!first) return "Blocked";
  const more = card.blockedBy.length - 1;
  return `Blocked: ${first.text}${more > 0 ? ` (+${more} more)` : ""}`;
}

const attachmentWords = (count: number) =>
  count === 1 ? "1 attachment" : `${count} attachments`;

// questionsWaiting says that agents' questions about a ticket wait for
// the user, when no live run of its own explains it (CARD-6).
const questionsWaiting = (count: number) =>
  count === 1 ? "Question waiting" : `${count} questions waiting`;

// cardName is the card's accessible name: everything the card can show,
// at any density, including the markers a narrow meta line drops.
export function cardName(
  card: Card,
  now: Date,
  {
    workstreamTitle,
    showProject,
  }: { workstreamTitle?: string; showProject?: string },
): string {
  const rejected = card.column === "done" && card.rejected === true;
  const questions =
    card.openQuestions > 0 && card.live?.state !== "needs-you"
      ? `needs you: ${questionsWaiting(card.openQuestions).toLowerCase()}`
      : "";
  const priority = priorityLabel(card.priority);
  const age = runningTime(card.modified, now);
  return [
    card.title,
    card.id,
    rejected ? "rejected" : "",
    card.blocked ? blockedWords(card).replace(/^Blocked/, "blocked") : "",
    card.needsRepair.length > 0 ? "needs repair" : "",
    card.live ? liveLabel(card.live) : "",
    questions,
    requestWords(card.changeRequests),
    card.type,
    priority ? `${priority.toLowerCase()} priority` : "",
    card.attachments > 0 ? attachmentWords(card.attachments) : "",
    card.hasReview ? "review written" : "",
    age ? `changed ${age} ago` : "",
    workstreamTitle ? `${workstreamTitle} workstream` : "",
    showProject ? `in ${showProject}` : "",
  ]
    .filter(Boolean)
    .join(", ");
}

// liveLabel is the live run in words, for the card's accessible name.
function liveLabel(live: Live): string {
  const state =
    live.state === "needs-you"
      ? `needs you: ${needsReason(live.reason)}`
      : STATE_LABEL[live.state].toLowerCase();
  const plan =
    live.total > 0
      ? `, plan ${live.done} of ${live.total}${live.step ? `: ${live.step}` : ""}`
      : "";
  return `${agentName(live.agent)} ${state}${plan}`;
}

// RunLine is Detailed's live run in words (VIEW-8): its state, the agent
// and the time since its last activity, then the plan step when it has a
// plan.
function RunLine({ live, now }: { live: Live; now: Date }) {
  const step =
    live.total > 0
      ? `${live.done}/${live.total}${live.step ? ` · ${live.step}` : ""}`
      : "";
  return (
    <span
      data-row="run"
      data-live={live.state}
      className="flex min-w-0 flex-col gap-0.5 text-2xs"
    >
      <span className="flex min-w-0 items-center gap-1.5">
        <RunStateMark state={live.state} size={10} className="text-ink" />
        <span className="shrink-0 font-medium text-ink">
          {STATE_LABEL[live.state]}
        </span>
        <span className="shrink-0 text-ink-muted">{agentName(live.agent)}</span>
        <span
          className="ml-auto shrink-0 text-ink-muted"
          title={`Last activity ${absoluteTime(live.lastActivity)}`}
        >
          {runningTime(live.lastActivity, now)}
        </span>
      </span>
      {step ? (
        <span className="truncate text-ink" title={live.step}>
          {step}
        </span>
      ) : null}
    </span>
  );
}
