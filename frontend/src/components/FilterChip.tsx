import type { KeyboardEvent, MouseEvent, ReactNode } from "react";

import type { ChoiceState } from "../model/filters";

// wantsExclude is a click or key press with Cmd (macOS) or Ctrl held, which
// hides a value instead of showing only it (FH-39).
export function wantsExclude(event: { metaKey: boolean; ctrlKey: boolean }) {
  return event.metaKey || event.ctrlKey;
}

const modifier = () =>
  typeof navigator !== "undefined" &&
  /Mac|iPhone|iPad/.test(navigator.userAgent)
    ? "⌘"
    : "Ctrl";

// filterHint is the tooltip of a filter chip or menu item.
export function filterHint(name: string, state: ChoiceState) {
  if (state === "included") return `Showing only ${name}. Click to restore.`;
  if (state === "excluded") return `Hiding ${name}. Click to restore.`;
  return `Show only ${name}. ${modifier()}-click hides it.`;
}

// filterHandlers make a button toggle a filter value: a click shows only it,
// a Cmd or Ctrl click (or Enter or Space with either held) hides it, and a
// click on a chosen value restores it.
export function filterHandlers(onToggle: (exclude: boolean) => void) {
  return {
    onClick: (event: MouseEvent) => onToggle(wantsExclude(event)),
    onKeyDown: (event: KeyboardEvent) => {
      if ((event.key === "Enter" || event.key === " ") && wantsExclude(event)) {
        event.preventDefault();
        onToggle(true);
      }
    },
  };
}

const looks: Record<ChoiceState, string> = {
  idle: "border-rule bg-card text-ink-muted shadow-card hover:border-ink-muted hover:text-ink",
  included:
    "border-select bg-select-surface text-ink shadow-[0_0_0_1px_var(--fh-select)]",
  excluded:
    "border-dashed border-ink-muted bg-transparent text-ink-faint line-through hover:text-ink-muted",
};

interface FilterChipProps {
  // name is the value's words, shown on the chip.
  name: string;
  // bullet is the round mark before the name: a workstream's line bullet
  // or a Colour by paint.
  bullet: ReactNode;
  state: ChoiceState;
  // hidden takes a chip that does not fit its row out of view and out of
  // the tab order, keeping its space so the row can be measured.
  hidden?: boolean;
  onToggle(exclude: boolean): void;
}

// FilterChip is the one rounded filter chip of the Board's chip row (FH-39),
// for workstreams and the colour key alike: a bullet and the name.
// Shown-only values take the selection ring; hidden values are dashed and
// struck through, as suspended service is.
export function FilterChip({
  name,
  bullet,
  state,
  hidden,
  onToggle,
}: FilterChipProps) {
  return (
    <button
      type="button"
      aria-pressed={state === "included"}
      aria-label={state === "excluded" ? `${name}, hidden` : name}
      aria-hidden={hidden || undefined}
      tabIndex={hidden ? -1 : undefined}
      title={filterHint(name, state)}
      {...filterHandlers(onToggle)}
      className={`flex shrink-0 items-center gap-1.5 rounded-full border py-0.5 pr-2.5 pl-0.5 text-xs whitespace-nowrap transition-[color,border-color] ${looks[state]} ${hidden ? "invisible" : ""}`}
    >
      {bullet}
      <span className="font-semibold">{name}</span>
    </button>
  );
}
