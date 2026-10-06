# Frontend UI facts

- Approved next visual direction (being implemented): Metro Pop light and
  Night Service black/neutral charcoal dark. Both follow one layout and
  content contract, `docs/dev/specs/ui-layout.md`; dark mode changes palette
  only. See `docs/dev/roadmap-items/metro-theme-rollout.md` and D18. The
  Transit Line Map details below describe the current implementation until
  rollout.

- Framework: React with strict TypeScript, built by Vite into static assets
  embedded in the Go binary. Root `frontend/`, source `frontend/src/`.
- Styling: Tailwind CSS v4. Design tokens (colour, spacing, type, radius,
  focus, motion) are CSS custom properties defined once in
  `src/styles/tokens.css` and exposed to Tailwind through `@theme`; components
  use token-backed utilities, never raw hex values. Light and dark themes via
  a `data-theme` attribute set from the saved preference (system for now).
- Layers and dependency direction: `styles/` (tokens) and `model/` (pure
  view models: line assignment, filters, paint, remembered scopes, times,
  markdown links, grid movement) → `components/` (primitives and shared
  patterns: Button, Icon, LineBullet, StateNote, Tabs, SidePanel,
  SegmentedControl, fields and form fields, Dialog, Toast, Markdown,
  RouteBar, EmptyState, RunState, PlanRoute, QuestionCard) → `features/<feature>/` (board,
  card, editing, agents, workstreams, table, projects, filters; later
  settings) → `app/`
  (shell, hash routing, composition). Shared layers never import features
  or the shell.
- `src/lifecycle/` owns the single Singleserve session (`connect` from
  `/_singleserve/client.js`), heartbeat, backend-lost and terminal states.
  `src/api/` owns typed requests via `session.fetch` only. `src/state/` owns
  loading and freshness: `useRevision` long-polls `/api/changes` and
  `useResource` keeps the last good value and reloads when the revision
  moves; `usePreferences` saves UI preferences through the backend.
- Editing: every edit from the panel carries the ticket's content hash; a
  409 opens the conflict dialog. Moves from the board set only `status`.
  Native `<dialog>` for decisions (blocked move, conflict, new ticket); a
  polite toast with Undo for results. Drag and drop uses `@dnd-kit` pointer
  sensors; the keyboard alternative is Shift with an arrow, and the panel's
  Move to menu.
- Tickets are named by id (`FH-42`) everywhere: cards, panel, table,
  search and the `?t=<id>` route parameter. Bare ids of known project keys in
  markdown link to the ticket (`model/markdown.ts`, KEY-3). A board with v1
  projects shows the migrate command instead of guessing.
- Prohibited: raw `fetch` to the backend, credentials in JavaScript, Web
  Storage for anything, permissive CORS, routes under `/_singleserve/`, raw
  HTML in markdown.
- Drag and drop: `@dnd-kit` with keyboard sensors; every drag has a menu
  alternative. Markdown: `react-markdown` + `remark-gfm`.
- Desktop first (1280 px and up); narrow widths show one column with a column
  picker. Healthy connection state is visually quiet.
- Visual direction (approved 2026-10-04 with the `impeccable` skill, D14):
  **Transit Line Map**, Vignelli diagram and Unimark signage. Black signage
  band; white map ground (charcoal at night); Archivo Variable self-hosted,
  tabular numerals; columns and sections as station signs (heavy top rule);
  round line bullets and a card-edge stripe in the MTA palette always mean a
  workstream; a muted paint palette shows the board's "Colour by" attribute
  (type, priority, age or none) as a tinted card header plus a named tag;
  black band and black project rail frame a dotted platform ground with
  shadowed cards (board-refresh, 2026-10-05); ticket state is ink, shape and words (diamond = blocked by a
  dependency, quiet text = waiting on the line's order, dashed border plus
  hatched band = needs repair); run state is ink and shape too (Needs you is
  the one inverted plate; a Working run's dot beats slowly, the board's only
  motion, off under reduced motion); Workstreams drawn as transit lines. The
  contract lives in `.impeccable/surfaces/`; `DESIGN.md` records the built
  system. Icons are an authored SVG set in `components/Icon.tsx`.
- Validation: Vitest for view-models and components; Playwright screenshots
  at 1280 and 1920 in light and dark with axe-core for visible changes.
