import type { ReactNode } from "react";

// The Overview's section card (DESIGN.md "Overview Section Card"): shared by
// the ticket sections and the projects list, so they read as one family.

// sectionCardClass is the rounded card every section sits on.
export const sectionCardClass =
  "overflow-hidden rounded-panel border border-rule bg-card shadow-card";

// sectionHeadClass lays out a head: its mark, title and count badge in the
// display head cut.
export const sectionHeadClass =
  "flex w-full items-center gap-3 px-4 py-3 text-left text-xl leading-tight display-cut";

// quietDisc is a mark with no role colour: an ink icon on a card disc.
export const quietDisc = "bg-card text-ink shadow-card";

// SectionDisc is a head's round mark; disc is its colour pair.
export function SectionDisc({
  disc,
  children,
}: {
  disc: string;
  children: ReactNode;
}) {
  return (
    <span
      aria-hidden="true"
      className={`grid size-7 shrink-0 place-items-center rounded-full ${disc}`}
    >
      {children}
    </span>
  );
}

// CountBadge is a head's round count.
export function CountBadge({ count }: { count: number }) {
  return (
    <span className="grid h-7 min-w-7 shrink-0 place-items-center rounded-full bg-card px-2 text-sm font-semibold tabular-nums text-ink shadow-card">
      {count}
    </span>
  );
}
