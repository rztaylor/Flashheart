import type { CSSProperties, ReactNode } from "react";

import { type Paint, paintVars } from "../model/paint";
import { type StateTone, statusOf } from "../model/status";
import { Icon, type IconName } from "./Icon";

const toneClass: Record<StateTone, string> = {
  neutral: "bg-(--fh-state-neutral) text-(--fh-state-neutral-ink)",
  progress: "bg-(--fh-state-progress) text-(--fh-state-progress-ink)",
  review: "bg-(--fh-state-review) text-(--fh-state-review-ink)",
  done: "bg-(--fh-state-done) text-(--fh-state-done-ink)",
  blocked: "bg-(--fh-state-blocked) text-(--fh-state-blocked-ink)",
};

const toneIcon: Record<StateTone, IconName> = {
  neutral: "ring",
  progress: "half",
  review: "review",
  done: "check",
  blocked: "diamond",
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
      className={`${pillBase} text-2xs ${toneClass[tone]} ${className ?? ""}`}
    >
      <Icon name={icon ?? toneIcon[tone]} size={11} />
      {children}
    </span>
  );
}

// StatusPill names a ticket's column (or an archived or missing station).
export function StatusPill({
  column,
  className,
}: {
  column: string;
  className?: string;
}) {
  const { tone, label } = statusOf(column);
  return (
    <Pill tone={tone} className={className}>
      {label}
    </Pill>
  );
}

// BlockerPill is a real blocker: the diamond and the reason in words. Long
// reasons wrap within the pill rather than running off the card.
export function BlockerPill({
  children,
  className,
}: {
  children: string;
  className?: string;
}) {
  return (
    <span
      data-tone="blocked"
      className={`inline-flex max-w-full items-start gap-1.5 rounded-control px-2 py-1 text-xs font-medium ${toneClass.blocked} ${className ?? ""}`}
    >
      <Icon name="diamond" size={12} className="mt-[0.15em]" />
      <span className="line-clamp-3">
        <span className="sr-only">Blocked: </span>
        {children}
      </span>
    </span>
  );
}

// Tag is a ticket attribute (type, priority, age). The one chosen in
// "Colour by" is filled with its paint; the rest are neutral outlined chips.
export function Tag({
  paint,
  strong,
  children,
}: {
  paint?: Paint;
  // strong sets the word in semibold (High priority).
  strong?: boolean;
  children: ReactNode;
}) {
  if (paint) {
    return (
      <span
        data-paint={paint.token}
        style={paintVars(paint) as CSSProperties}
        className="inline-flex items-center rounded-full bg-(--paint) px-2 text-2xs leading-5 font-semibold text-(--paint-ink)"
      >
        {children}
      </span>
    );
  }
  return (
    <span
      className={`inline-flex items-center rounded-full border border-rule px-2 text-2xs leading-[1.125rem] ${strong ? "font-semibold text-ink" : "text-ink-muted"}`}
    >
      {children}
    </span>
  );
}
