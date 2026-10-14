import type { ReactNode } from "react";

// EmptyState explains an empty view and what fills it; mark sits centred
// above the title. heading makes the title the page's h1, for a page that
// has nothing else to name it.
export function EmptyState({
  title,
  mark,
  heading = false,
  children,
}: {
  title: string;
  mark?: ReactNode;
  heading?: boolean;
  children?: ReactNode;
}) {
  const Title = heading ? "h1" : "p";
  return (
    <div className="mx-auto max-w-md px-4 py-16 sm:px-0 text-center">
      {mark ? <div className="mb-3 flex justify-center">{mark}</div> : null}
      <Title className="text-xl display-cut">{title}</Title>
      {children ? (
        <div className="mt-2 text-sm text-ink-muted">{children}</div>
      ) : null}
    </div>
  );
}
