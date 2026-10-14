# State boundary

Owns how board data is loaded and kept fresh in the browser: `useRevision`
long-polls the board revision (LIFE-3) and `useResource` loads through the
authenticated fetch, keeps the last good value while reloading, and reloads
when the revision moves or the page becomes visible; `useNow` ticks running
times between revisions. `usePreferences` loads and saves UI preferences
through the backend (CFG-2), coalescing a burst of changes into one save and
flushing it, kept alive, when the page is hidden or unloaded. `leaveGuard` is the one guard on leaving the page
with unsaved work (FH-66): its holder is asked before the router moves, and
the browser prompts on reload; a save in flight holds only the browser's
prompt (`guardUnloading`, FH-156, D70).

Does not own request shapes (`api/`), the Singleserve session
(`lifecycle/`), routing (`app/`), view models (`model/`) or presentation.
Never uses Web Storage.
