import type { ReactNode } from "react";

import { Button } from "../components/Button";
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

// PageActions are the header's far end: the Archive link with the scope's
// archived count, and New ticket, each absent where the view has none.
export function PageActions({
  archive,
  onNewTicket,
}: {
  archive?: { href: string; count: number; onOpen(): void };
  onNewTicket?: () => void;
}) {
  return (
    <>
      {archive ? (
        <a
          href={archive.href}
          onClick={(event) => {
            event.preventDefault();
            archive.onOpen();
          }}
          className="flex items-center gap-1.5 rounded-control px-2 py-1 text-xs text-ink-muted transition-colors hover:bg-well hover:text-ink"
        >
          <Icon name="archive" size={14} />
          Archive
          <span className="tabular-nums">{archive.count}</span>
        </a>
      ) : null}
      {onNewTicket ? (
        <Button variant="primary" className="h-9 py-0" onClick={onNewTicket}>
          <Icon name="plus" size={16} />
          New ticket
        </Button>
      ) : null}
    </>
  );
}
