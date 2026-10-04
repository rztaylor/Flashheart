import { forwardRef, type KeyboardEvent } from "react";

import { type Card, splitReasons } from "../../api/board";
import { Icon } from "../../components/Icon";
import { LineBullet } from "../../components/LineBullet";
import { StateNote } from "../../components/StateNote";
import type { Line } from "../../model/lines";
import { runningTime } from "../../model/time";
import type { Density } from "../filters/FilterBar";

interface TicketCardProps {
  card: Card;
  line?: Line;
  workstreamTitle?: string;
  density: Density;
  dimmed?: boolean;
  showProject?: string;
  selected?: boolean;
  tabIndex: number;
  now: Date;
  onOpen(): void;
  onKeyDown(event: KeyboardEvent<HTMLButtonElement>): void;
  onFocus(): void;
}

const priorityLabel: Record<string, string> = {
  high: "High",
  medium: "Medium",
  low: "Low",
};

// TicketCard is one ticket on the board (VIEW-6). Compact: line bullet,
// title, type and priority. Normal adds the blocked proof and criteria.
// Detailed adds the excerpt, handoff "next" and attachments. Real blockers
// are marked; waits on an earlier station of the line are quiet.
export const TicketCard = forwardRef<HTMLButtonElement, TicketCardProps>(
  function TicketCard(
    {
      card,
      line,
      workstreamTitle,
      density,
      dimmed,
      showProject,
      selected,
      tabIndex,
      now,
      onOpen,
      onKeyDown,
      onFocus,
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
    return (
      <button
        ref={ref}
        type="button"
        tabIndex={tabIndex}
        onClick={onOpen}
        onKeyDown={onKeyDown}
        onFocus={onFocus}
        aria-current={selected ? "true" : undefined}
        aria-label={`${card.title}, ${card.slug}${card.blocked ? ", blocked" : ""}${repair ? ", needs repair" : ""}`}
        className={`group relative flex w-full shrink-0 flex-col gap-1.5 overflow-hidden rounded-card border bg-card px-3 py-2.5 text-left transition-[border-color,opacity,box-shadow] duration-150 hover:border-ink-muted ${
          selected
            ? "border-rule-strong shadow-[0_0_0_1px_var(--fh-rule-strong)]"
            : repair
              ? "border-dashed border-ink-muted"
              : "border-rule"
        } ${dimmed ? "opacity-35" : ""} ${repair ? "pt-3.5" : ""}`}
      >
        {repair ? (
          <span
            aria-hidden="true"
            className="hatched-strong absolute inset-x-0 top-0 h-1.5"
          />
        ) : null}
        <span className="flex items-start gap-2">
          <LineBullet line={line} size="sm" label={workstreamTitle} />
          <span
            className={`min-w-0 flex-1 text-sm leading-snug font-medium ${done ? "text-ink-muted" : "text-ink"}`}
          >
            {card.title}
          </span>
          {age ? (
            <span
              className="shrink-0 text-2xs text-ink-faint"
              title={`Last changed ${card.modified}`}
            >
              {age}
            </span>
          ) : null}
        </span>

        <span className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-2xs text-ink-muted">
          {showProject ? (
            <span className="font-medium text-ink">{showProject}</span>
          ) : null}
          <span className="truncate font-mono">{card.slug}</span>
          {card.priority ? (
            <span
              className={
                card.priority === "high" ? "font-semibold text-ink" : undefined
              }
            >
              {priorityLabel[card.priority] ?? card.priority}
            </span>
          ) : null}
          {density === "compact" && blocker ? (
            <span className="flex items-center gap-0.5 font-semibold text-ink">
              <Icon name="diamond" size={10} />
              Blocked
            </span>
          ) : null}
        </span>

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
      </button>
    );
  },
);
