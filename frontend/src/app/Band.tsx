import { useState } from "react";

import { Button } from "../components/Button";
import { SearchField } from "../components/Field";
import { Icon, type IconName } from "../components/Icon";
import { StaleNote } from "../components/StaleNote";
import type { SingleserveLifecycle } from "../lifecycle/useSingleserve";
import { nextRemark } from "../model/remarks";
import { BackendStatus } from "./BackendStatus";
import { NeedsYouPill } from "./NeedsYouPill";
import type { View } from "./route";

// BandView is a view the band's tabs open.
export type BandView = Exclude<View, "archive" | "ticket">;

const views: { id: BandView; label: string; icon: IconName }[] = [
  { id: "overview", label: "Overview", icon: "overview" },
  { id: "workstreams", label: "Workstreams", icon: "lines" },
  { id: "table", label: "Table", icon: "table" },
  { id: "board", label: "Board", icon: "board" },
];

// Band is the signage band (ui-layout.md §1): the bolt and wordmark, whose
// tooltip is a marginal remark, the view tabs, the Needs you plate, search
// where the view has it, the backend status and Quit.
export function Band({
  lifecycle,
  view,
  onView,
  needsYou,
  needsYouOn,
  onNeedsYou,
  search,
}: {
  lifecycle: SingleserveLifecycle;
  view: View;
  onView(view: BandView): void;
  // needsYou counts the runs that need the user, across every project.
  needsYou: number;
  needsYouOn: boolean;
  onNeedsYou(): void;
  // search is the ticket search, on the Board and Table only.
  search?: { query: string; onChange(query: string): void };
}) {
  const [brand] = useState(() => nextRemark("brand").text);
  const { state, ready } = lifecycle;
  const stopping = state.phase === "stopping";
  return (
    <header className="relative flex min-w-0 items-center gap-1.5 overflow-hidden bg-band px-3 text-on-band sm:gap-6 sm:px-5">
      <span className="flex items-center gap-2">
        <Bolt />
        <span className="wordmark text-xl max-sm:sr-only" title={brand}>
          Flashheart
        </span>
      </span>
      <nav aria-label="Views" className="flex items-center gap-0.5 sm:gap-1">
        {views.map((item) => {
          const active = view === item.id;
          return (
            <a
              key={item.id}
              href={`#${item.id}`}
              aria-current={active ? "page" : undefined}
              onClick={(event) => {
                event.preventDefault();
                onView(item.id);
              }}
              className={`flex h-9 items-center gap-2 rounded-control px-2 text-sm font-semibold transition-colors focus-visible:outline-on-band sm:px-3 ${
                active
                  ? "bg-band-tab-active text-on-band-tab-active shadow-card"
                  : "text-on-band-muted hover:bg-band-field hover:text-on-band"
              }`}
            >
              <Icon name={item.icon} size={16} />
              <span className="max-sm:sr-only">{item.label}</span>
            </a>
          );
        })}
      </nav>
      <div className="ml-auto flex shrink-0 items-center gap-1 sm:gap-3">
        <NeedsYouPill count={needsYou} on={needsYouOn} onToggle={onNeedsYou} />
        {search ? (
          <div className="hidden md:flex">
            <SearchField
              label="Search tickets"
              placeholder="Search id, title, text, tags"
              value={search.query}
              onChange={search.onChange}
            />
          </div>
        ) : null}
        <BackendStatus
          state={state}
          checking={lifecycle.checking}
          onCheck={() => void lifecycle.checkHealth()}
        />
        <Button
          variant="band"
          className="h-9 py-0 max-sm:px-2"
          onClick={() => void lifecycle.quit()}
          disabled={!ready || stopping}
        >
          <Icon name="power" size={14} />
          <span className="max-sm:sr-only">
            {stopping ? "Quitting…" : "Quit"}
          </span>
        </Button>
      </div>
    </header>
  );
}

// BandNotices sit under the band: Quit's denial, and a reload of the
// projects or the board that failed while the last good copy shows.
export function BandNotices({
  shutdownDenied,
  onDismiss,
  stale,
}: {
  shutdownDenied?: string;
  onDismiss(): void;
  stale?: string;
}) {
  return (
    <div>
      {shutdownDenied ? (
        <div
          role="alert"
          className="flex items-center justify-between gap-4 border-b border-rule bg-danger-surface px-4 py-2 text-sm"
        >
          <span>
            <strong className="font-semibold">Flashheart did not quit.</strong>{" "}
            {shutdownDenied}
          </span>
          <Button variant="quiet" onClick={onDismiss}>
            Dismiss
          </Button>
        </div>
      ) : null}
      {stale ? (
        <StaleNote
          error={stale}
          className="border-b border-rule bg-well px-4 py-1.5"
        />
      ) : null}
    </div>
  );
}

// Bolt is the brand mark beside the wordmark: the one flash of signal yellow.
function Bolt() {
  return (
    <svg
      viewBox="0 0 16 20"
      width={13}
      height={16}
      aria-hidden="true"
      className="shrink-0 text-flash"
    >
      <path d="M10 0 1 11.5h6L5.5 20 15 7.5H8.8L10 0Z" fill="currentColor" />
    </svg>
  );
}
