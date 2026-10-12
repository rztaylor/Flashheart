import type { ComponentProps, ReactNode } from "react";

// Table is the one table look (ui-layout.md §6): a rounded card surface
// whose head sticks to the top of its scroller, with head cells and rows
// below. The scroller and the cells' content belong to the view.
export function Table({
  caption,
  head,
  children,
}: {
  // caption names the table for screen readers.
  caption?: string;
  // head is the head row's cells (HeadCell).
  head: ReactNode;
  // children are the body rows (Row).
  children: ReactNode;
}) {
  return (
    <table className="w-full border-separate border-spacing-0 overflow-hidden rounded-panel border border-rule bg-card text-sm shadow-card">
      {caption ? <caption className="sr-only">{caption}</caption> : null}
      <thead className="sticky top-0 z-10 bg-column">
        <tr>{head}</tr>
      </thead>
      <tbody>{children}</tbody>
    </table>
  );
}

// HeadCell is one column's head.
export function HeadCell({
  className,
  children,
  ...props
}: ComponentProps<"th">) {
  return (
    <th
      scope="col"
      {...props}
      className={`border-b border-rule px-3 py-2.5 text-left text-sm heading-cut ${className ?? ""}`}
    >
      {children}
    </th>
  );
}

// SkeletonTable is the table look while its rows load (ui-layout.md §8):
// the same surface and head band, with rows of placeholder bars.
export function SkeletonTable({ rows = 5 }: { rows?: number }) {
  return (
    <div
      aria-hidden="true"
      className="overflow-hidden rounded-panel border border-rule bg-card shadow-card"
    >
      <div className="h-10 border-b border-rule bg-column" />
      {Array.from({ length: rows }, (_, row) => (
        <div
          // biome-ignore lint/suspicious/noArrayIndexKey: placeholder rows have no identity.
          key={row}
          className="flex items-center gap-6 border-b border-rule px-3 py-3.5 last:border-b-0"
        >
          <div className="h-4 w-2/5 animate-pulse rounded-card bg-well" />
          <div className="h-4 w-20 animate-pulse rounded-full bg-well" />
          <div className="h-4 w-16 animate-pulse rounded-full bg-well" />
          <div className="ml-auto h-4 w-12 animate-pulse rounded-card bg-well" />
        </div>
      ))}
    </div>
  );
}

// Row is one body row: divided from the next, lit on hover, or marked as
// the selected one.
export function Row({
  selected = false,
  children,
}: {
  selected?: boolean;
  children: ReactNode;
}) {
  return (
    <tr
      className={`[&>td]:border-b [&>td]:border-rule last:[&>td]:border-b-0 ${selected ? "bg-select-surface" : "hover:bg-well"}`}
    >
      {children}
    </tr>
  );
}
