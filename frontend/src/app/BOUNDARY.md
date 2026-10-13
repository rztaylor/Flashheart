# App boundary

Owns the application shell and composition (`docs/dev/specs/ui-layout.md`
§1): the band with view tabs (Board, Overview, Workstreams, Table), the Needs
you plate and its Board and Table filter with the notice for runs no card
shows, which opens the Overview (VIEW-2), old Agents routes landing on the
Overview, New ticket in a chosen project (the Overview's Create ticket),
opening the panel on a tab (Review results), backend status and Quit, the page header with the scope's
identity, the view's summary line and the Archive link, the archive route
(a project's archived tickets, or the projects archive in All projects),
the ticket page route (`#/ticket/<id>`, a ticket's full page under the
band, CARD-7; a navigation waits on the leave guard while an edit is
unsaved)
and the Undo after archiving a project, the screen's one marginal remark
(`AsideProvider`) and wiring which moves earn one, the brand tooltip, the
shutdown-denial alert, the terminal stopped screen, theme
application to `<html data-theme>`, live updates and saved preferences
wired into the views (the view and filters remembered per project), and
wiring lifecycle, API and features together.

`Shell` is composition only; each concern has one owner beside it:

- `Band`: the band's markup (wordmark, view tabs, Needs you plate, search,
  status, Quit) and the notices under it.
- `useBoardData`: loading the projects and the scope's board, live
  reloads, which failed reload the band's last-good-copy note names
  (`staleError`), editing wired to them with pending moves placed at once, the
  move remark (`model/remarks` `moveRemark`), and the lines, workstreams
  and keys derived from the projects.
- `useScopeMemory` over `scopeMemory` (pure, tested): the filters, and the
  view and filters remembered per scope.
- `useTicketPanel`: opening the panel (on a tab) and returning focus to its
  opener when it closes.
- `ViewOutlet`: the routed view's body (read errors with Try again, each
  view's own skeleton while the projects load, the v1 notice
  (`V1Notice`), the missing-root state, archive, Overview, Workstreams, and
  the Board or Table with their toolbar, filter options and paints).
- `PageHeader` with `PageActions`, `NarrowControls` (the phone control row)
  and `RailFooter` (theme, board root, version).

Does not own the Singleserve session (`lifecycle/`), request contracts
(`api/`), shared primitives (`components/`), loading state (`state/`) or
feature behaviour (`features/`). It is the only layer allowed to import
features.
