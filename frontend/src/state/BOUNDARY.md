# State boundary

Owns how board data is loaded and kept fresh in the browser: `useRevision`
long-polls the board revision (LIFE-3) and `useResource` loads through the
authenticated fetch, keeps the last good value while reloading, and reloads
when the revision moves or the page becomes visible. `usePreferences` loads
and saves UI preferences through the backend (CFG-2), coalescing a burst of
changes into one save of what changed from the server's copy, flushing a
pending save, kept alive, when the page is hidden or unloaded, and reading
the server's copy again on `refresh` (the shell calls it as the revision
moves), taken only when nothing of the page's is waiting (FH-173, D80). `leaveGuard` is the one guard on leaving the page
with unsaved work (FH-66): its holder is asked before the router moves, and
the browser prompts on reload; a save in flight holds only the browser's
prompt (`guardUnloading`, FH-156, D70).

Does not own request shapes (`api/`), the Singleserve session
(`lifecycle/`), routing (`app/`), view models (`model/`) or presentation.
Never uses Web Storage.
