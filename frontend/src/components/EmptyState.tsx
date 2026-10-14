import type { ReactNode } from "react";

// EmptyState explains an empty view and what fills it. heading makes the
// title the page's h1, for a page that has nothing else to name it.
export function EmptyState({
  title,
  heading = false,
  children,
}: {
  title: string;
  heading?: boolean;
  children?: ReactNode;
}) {
  const Title = heading ? "h1" : "p";
  return (
    <div className="mx-auto max-w-md py-16 text-center">
      <Title className="text-xl display-cut">{title}</Title>
      {children ? (
        <div className="mt-2 text-sm text-ink-muted">{children}</div>
      ) : null}
    </div>
  );
}
