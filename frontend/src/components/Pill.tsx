import type { ReactNode } from "react";

import { type StateTone, statusOf } from "../model/status";
import { Icon, type IconName } from "./Icon";

// stateToneClass and stateToneIcon are a state's colour role and its icon,
// for the pill and the Overview's pipeline (FH-63).
export const stateToneClass: Record<StateTone, string> = {
  neutral: "bg-(--fh-state-neutral) text-(--fh-state-neutral-ink)",
  progress: "bg-(--fh-state-progress) text-(--fh-state-progress-ink)",
  review: "bg-(--fh-state-review) text-(--fh-state-review-ink)",
  done: "bg-(--fh-state-done) text-(--fh-state-done-ink)",
  blocked: "bg-(--fh-state-blocked) text-(--fh-state-blocked-ink)",
};

export const stateToneIcon: Record<StateTone, IconName> = {
  neutral: "ring",
  progress: "half",
  review: "review",
  done: "check",
  blocked: "blocked",
};

const pillBase =
  "inline-flex max-w-full items-center gap-1 rounded-full px-2 leading-5 font-semibold whitespace-nowrap";

// Pill is a state in its colour role, always with an icon and words
// (ui-layout.md §7).
export function Pill({
  tone,
  icon,
  children,
  className,
}: {
  tone: StateTone;
  icon?: IconName;
  children: ReactNode;
  className?: string;
}) {
  return (
    <span
      data-tone={tone}
      className={`${pillBase} text-2xs ${stateToneClass[tone]} ${className ?? ""}`}
    >
      <Icon name={icon ?? stateToneIcon[tone]} size={11} />
      {children}
    </span>
  );
}

// StatusPill names a ticket's column (or an archived or missing station),
// or Rejected, with a cross rather than a check, for a rejected ticket in
// Done.
export function StatusPill({
  column,
  rejected,
  className,
}: {
  column: string;
  rejected?: boolean;
  className?: string;
}) {
  const { tone, label } = statusOf(column, rejected);
  const crossed = rejected && column === "done";
  return (
    <Pill
      tone={tone}
      icon={crossed ? "close" : undefined}
      className={className}
    >
      {label}
    </Pill>
  );
}

// BlockerPill is a real blocker: the no-entry sign and the reason in words.
// Long reasons wrap within the pill rather than running off the card.
export function BlockerPill({
  children,
  title,
  lines = 3,
  className,
}: {
  // children is the reason, as text or with its ticket ids linked; title
  // carries it as text for the tooltip.
  children: ReactNode;
  title?: string;
  // lines clamps a long reason; its full text stays in the tooltip.
  lines?: 1 | 2 | 3;
  className?: string;
}) {
  const clamp = { 1: "line-clamp-1", 2: "line-clamp-2", 3: "line-clamp-3" };
  return (
    <span
      data-tone="blocked"
      title={title ?? (typeof children === "string" ? children : undefined)}
      className={`inline-flex max-w-full items-start gap-1.5 rounded-control px-2 py-1 text-xs font-medium ${stateToneClass.blocked} ${className ?? ""}`}
    >
      <Icon name="blocked" size={12} className="mt-[0.15em]" />
      <span className={clamp[lines]}>
        <span className="sr-only">Blocked: </span>
        {children}
      </span>
    </span>
  );
}

// Tag is a ticket attribute (type, priority, a tag) as a neutral outlined
// chip, in the panel, the table and the archive. Cards draw type and
// priority as marks instead (components/TicketMarks, FH-147).
export function Tag({
  strong,
  children,
}: {
  // strong sets the word in semibold (High priority).
  strong?: boolean;
  children: ReactNode;
}) {
  return (
    <span
      className={`inline-flex items-center rounded-full border border-rule px-2 text-2xs leading-[1.125rem] ${strong ? "font-semibold text-ink" : "text-ink-muted"}`}
    >
      {children}
    </span>
  );
}
