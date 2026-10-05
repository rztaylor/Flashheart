# Frontend UI facts

- Framework: React with strict TypeScript, built by Vite into static assets
  embedded in the Go binary. Root `frontend/`, source `frontend/src/`.
- Styling: Tailwind CSS v4. Design tokens (colour, spacing, type, radius,
  focus, motion) are CSS custom properties defined once in
  `src/styles/tokens.css` and exposed to Tailwind through `@theme`; components
  use token-backed utilities, never raw hex values. Light and dark themes via
  a `data-theme` attribute set from the saved preference (system for now).
- Layers and dependency direction: `styles/` (tokens) and `model/` (pure
  view models: line assignment, filters, times, markdown links, grid
  movement) → `components/` (primitives and shared patterns: Button, Icon,
  LineBullet, StateNote, Tabs, SidePanel, SegmentedControl, fields, Markdown,
  RouteBar, EmptyState) → `features/<feature>/` (board, card, workstreams,
  table, projects, filters; later agents, settings) → `app/` (shell, hash
  routing, composition). Shared layers never import features or the shell.
- `src/lifecycle/` owns the single Singleserve session (`connect` from
  `/_singleserve/client.js`), heartbeat, backend-lost and terminal states.
  `src/api/` owns typed requests via `session.fetch` only. `src/state/` owns
  loading and freshness (`useResource`: last good value kept, 10 s refresh
  while visible, replaced by long-poll with board-editing).
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
  hatched band = needs repair); Workstreams drawn as transit lines. The
  contract lives in `.impeccable/surfaces/`; `DESIGN.md` records the built
  system. Icons are an authored SVG set in `components/Icon.tsx`.
- Validation: Vitest for view-models and components; Playwright screenshots
  at 1280 and 1920 in light and dark with axe-core for visible changes.
