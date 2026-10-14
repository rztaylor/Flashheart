# App boundary

Owns the application shell (`docs/dev/specs/ui-layout.md` §1): the band
(view tabs, Needs you plate and filter, search, status, settings, Quit),
the notice for runs no card shows (VIEW-2), the page header, hash routing
(views, archives, the ticket page, a renamed workstream's new slug),
opening the card panel, loading the projects and the scope's board with
live reloads, the view and filters remembered per scope, the theme, the
one aside, stray file drops, the stopped screen, and wiring lifecycle,
API, state and features together. Every navigation waits on the leave
guard.

`Shell` is composition only; each concern has one owner beside it:
`Band`, `useBoardData`, `useScopeMemory` over pure `scopeMemory`,
`useTicketPanel`, `ViewOutlet` (the routed view's body), `route`,
`PageHeader` and `SettingsMenu`.

Does not own the Singleserve session (`lifecycle/`), request contracts
(`api/`), shared primitives (`components/`), loading state (`state/`) or
feature behaviour (`features/`). It is the only layer allowed to import
features.
