import type { ReactNode } from "react";

import { Icon } from "../components/Icon";
import { KeyBadge } from "../components/KeyBadge";

// PageHeader opens every view with its scope's identity, the project's key
// badge and name as the page title (All projects in the all scope), one
// quiet summary line for the view (ui-layout.md §1) and, at the far end, a
// quiet way into the archive and New ticket (FH-39).
export function PageHeader({
  projectKey,
  title,
  summary,
  live,
  aside,
}: {
  // projectKey is absent in the all scope, which shows the board icon.
  projectKey?: string;
  title: string;
  summary?: ReactNode;
  // live announces changes (a filtered ticket count); off where the line
  // follows live updates (Agents) and would chatter.
  live?: boolean;
  aside?: ReactNode;
}) {
  return (
    <header className="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-1 px-4 pt-5 pb-3 md:px-6">
      <h1 className="flex min-w-0 items-center gap-3 text-3xl display-cut">
        <KeyBadge size="lg">
          {projectKey ?? <Icon name="board" size={20} />}
        </KeyBadge>
        <span className="truncate" title={title}>
          {title}
        </span>
      </h1>
      {summary ? (
        <p
          className="text-sm text-ink-muted md:mt-2 md:self-start"
          aria-live={live ? "polite" : undefined}
        >
          {summary}
        </p>
      ) : null}
      {aside ? (
        <div className="ml-auto flex items-center gap-3 md:self-start">
          {aside}
        </div>
      ) : null}
    </header>
  );
}
