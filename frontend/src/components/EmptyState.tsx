import type { ReactNode } from "react";

// EmptyState explains an empty view and what fills it; mark sits centred
// above the title.
export function EmptyState({
  title,
  mark,
  children,
}: {
  title: string;
  mark?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className="mx-auto max-w-md px-4 py-16 sm:px-0 text-center">
      {mark ? <div className="mb-3 flex justify-center">{mark}</div> : null}
      <p className="text-xl display-cut">{title}</p>
      {children ? (
        <div className="mt-2 text-sm text-ink-muted">{children}</div>
      ) : null}
    </div>
  );
}
