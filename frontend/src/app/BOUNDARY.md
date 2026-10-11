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
band, CARD-7)
and the Undo after archiving a project, the screen's one marginal remark
(`AsideProvider`) and which moves earn one, the brand tooltip, the
shutdown-denial alert, the terminal stopped screen, theme
application to `<html data-theme>`, live updates and saved preferences
wired into the views (the view and filters remembered per project), and
wiring lifecycle, API and features together.

Does not own the Singleserve session (`lifecycle/`), request contracts
(`api/`), shared primitives (`components/`), loading state (`state/`) or
feature behaviour (`features/`). It is the only layer allowed to import
features.
