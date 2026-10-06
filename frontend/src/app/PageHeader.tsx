import type { ReactNode } from "react";

import { Icon } from "../components/Icon";

// PageHeader opens every view with its scope's identity, the project's key
// badge and name as the page title (All projects in the all scope), and one
// quiet summary line for the view (ui-layout.md §1).
export function PageHeader({
  projectKey,
  title,
  summary,
}: {
  // projectKey is absent in the all scope, which shows the board icon.
  projectKey?: string;
  title: string;
  summary?: ReactNode;
}) {
  return (
    <header className="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-1 px-4 pt-5 pb-3 md:px-6">
      <h1 className="flex min-w-0 items-center gap-3 text-2xl display-cut">
        <span
          aria-hidden="true"
          className="grid h-10 min-w-10 shrink-0 place-items-center rounded-control bg-rail-active px-2 text-lg text-on-rail-active shadow-[inset_0_0_0_1.5px_var(--fh-rail-active-mark)]"
        >
          {projectKey ?? <Icon name="board" size={20} />}
        </span>
        <span className="truncate" title={title}>
          {title}
        </span>
      </h1>
      {summary ? (
        <p
          className="text-sm text-ink-muted md:mt-2 md:self-start"
          aria-live="polite"
        >
          {summary}
        </p>
      ) : null}
    </header>
  );
}
