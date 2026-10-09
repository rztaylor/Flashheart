import type { ReactNode } from "react";

// The column well (ui-layout.md §2): the rounded surface of a board column.
// Callers add the border colour (rule, or select while a drop hovers).
export const wellSurface =
  "flex min-h-0 flex-col rounded-panel border bg-column p-2.5";

// WellHead heads a well: a round count badge (first visually, last in the
// heading's words) and the title in the display cut. titleId names the
// title alone, for a region labelled by it. A sticky head stays at the top
// of a well that scrolls with its neighbours, so the board keeps its column
// names and counts in view.
export function WellHead({
  id,
  titleId,
  title,
  count,
  sticky = false,
}: {
  id?: string;
  titleId?: string;
  title: string;
  count: ReactNode;
  sticky?: boolean;
}) {
  return (
    <h2
      id={id}
      className={`flex items-center gap-2.5 px-1 pt-1 pb-3 text-xl leading-tight display-cut ${
        sticky
          ? "sticky -top-1 z-[1] -mx-2.5 -mt-2.5 rounded-t-panel bg-column px-3.5 pt-3.5"
          : ""
      }`}
    >
      <span id={titleId}>{title}</span>
      <span className="order-first grid h-7 min-w-7 shrink-0 place-items-center rounded-full bg-card px-2 text-sm font-semibold tabular-nums text-ink shadow-card">
        {count}
      </span>
    </h2>
  );
}

// EmptySlot is an empty well's dashed slot with one quiet sentence.
export function EmptySlot({ children }: { children: ReactNode }) {
  return (
    <p className="rounded-card border border-dashed border-rule px-3 py-5 text-center text-xs text-ink-muted">
      {children}
    </p>
  );
}
