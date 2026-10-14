# Workstreams feature boundary

Owns the Workstreams view (VIEW-4, `docs/dev/specs/ui-layout.md` §4): each
workstream as a route card with its stations on a railway graph
(`graph.ts`), independent stations below, status, progress and notes;
collapsing a finished one (`collapse.ts`); opening stations in the card
panel, reordering independent ones (EDIT-4); and its load states.

Owns the New workstream dialog (EDIT-14) and the workstream page
(EDIT-13): its head, stations and sections edited in place on the hash
they opened on, and its name (FH-167), telling the shell the new slug to
follow. A workstream renamed or removed elsewhere is followed to what
`successor.ts` finds it became, else the page says it is gone (FH-174).

Does not own line colours (`model/lines`), which sections show
(`model/workstreamSections`), slugs, templates and rewrite rules
(backend), expanded choices and routing (`app/`), the card panel
(`card/`), in-place editing, conflicts and toasts (`editing/`) or the
editor (`editor/`).
