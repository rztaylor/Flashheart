# App boundary

Owns the application shell and composition (`docs/dev/specs/ui-layout.md`
§1): the band with view tabs (Board, Agents, Workstreams, Table), the Needs
you plate, backend status and Quit, the page header with the scope's
identity and the view's summary line, the shutdown-denial alert, the terminal stopped screen, theme
application to `<html data-theme>`, live updates and saved preferences
wired into the views (the view and filters remembered per project), and
wiring lifecycle, API and features together.

Does not own the Singleserve session (`lifecycle/`), request contracts
(`api/`), shared primitives (`components/`), loading state (`state/`) or
feature behaviour (`features/`). It is the only layer allowed to import
features.
