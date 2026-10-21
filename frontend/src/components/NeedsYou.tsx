import type { ReactNode } from "react";
import { STATE_LABEL } from "../model/runs";
import { RunStateMark } from "./RunState";

// attentionClass is the attention colour pair, the one plate's: nothing
// outside this file sets it (ui-layout.md §7).
export const attentionClass = "bg-attention text-on-attention";

const discSize = { sm: "size-5", lg: "size-7" } as const;
const discMark = { sm: 8, lg: 11 } as const;

// AttentionDisc is the round attention mark beside a heading or note: the
// Needs you disc, or another mark (a question's warning) in its place.
export function AttentionDisc({
  size = "sm",
  children,
}: {
  size?: keyof typeof discSize;
  children?: ReactNode;
}) {
  return (
    <span
      aria-hidden="true"
      className={`grid ${discSize[size]} shrink-0 place-items-center rounded-full ${attentionClass}`}
    >
      {children ?? (
        <RunStateMark state="needs-you" size={discMark[size]} still />
      )}
    </span>
  );
}

const plateSizes = {
  inline: "inline-flex gap-1 px-2 leading-5",
  band: "flex h-8 gap-1.5 px-2.5 text-xs sm:h-9 sm:px-3.5 sm:text-sm",
} as const;

export type PlateSize = keyof typeof plateSizes;

// plateMark is the Needs you disc's size on each plate.
export const plateMark: Record<PlateSize, number> = {
  inline: 9,
  band: 11,
};

// plateClass is the Needs you plate's look, for an element that must be
// something else too (the band's plate is a toggle button). pressed rings
// a band plate that filters the board.
export function plateClass(size: PlateSize, pressed = false): string {
  return `${plateSizes[size]} items-center rounded-full font-semibold whitespace-nowrap ${attentionClass}${
    pressed ? " shadow-[inset_0_0_0_2px_var(--fh-on-attention)]" : ""
  }`;
}

// NeedsYouPlate is Needs you as a plate in the attention colour, so it
// reads from across the room: a card or row's run state. The band's is
// NeedsYouPill; a project's on the rail is NeedsYouCount.
export function NeedsYouPlate({
  size = "inline",
  className,
  children,
}: {
  size?: Exclude<PlateSize, "band">;
  className?: string;
  // children replace the words: a count and its phrase.
  children?: ReactNode;
}) {
  return (
    <span className={`${plateClass(size)} ${className ?? ""}`}>
      <RunStateMark state="needs-you" size={plateMark[size]} />
      {children ?? STATE_LABEL["needs-you"]}
    </span>
  );
}

// NeedsYouCount is a project's Needs you as a count on the corner of its
// rail badge, in the attention colour (FH-150). Decorative: the badge's
// name says how many agents need the user.
export function NeedsYouCount({ count }: { count: number }) {
  return (
    <span
      aria-hidden="true"
      data-mark="needs-you"
      className={`absolute -top-1.5 -right-2 grid h-4 min-w-4 place-items-center rounded-full px-1 text-2xs leading-none font-semibold tabular-nums ring-2 ring-rail ${attentionClass}`}
    >
      {/* A space after the badge's key for tools that read the text
          (axe joins elements' text as it is); grid drops it. */}{" "}
      {count}
    </span>
  );
}
