import type { ReactNode } from "react";

// EmptyState explains an empty view and what fills it.
export function EmptyState({
  title,
  children,
}: {
  title: string;
  children?: ReactNode;
}) {
  return (
    <div className="mx-auto max-w-md py-16 text-center">
      <p className="text-xl display-cut">{title}</p>
      {children ? (
        <div className="mt-2 text-sm text-ink-muted">{children}</div>
      ) : null}
    </div>
  );
}
