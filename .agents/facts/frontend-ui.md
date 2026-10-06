# Frontend UI facts

- Framework: React with strict TypeScript, built by Vite into static assets
  embedded in the Go binary. Root `frontend/`, source `frontend/src/`.
- Styling: Tailwind CSS v4. Design tokens (colour, spacing, type, radius,
  focus, motion) are CSS custom properties defined once in
  `src/styles/tokens.css` and exposed to Tailwind through `@theme`; components
  use token-backed utilities, never raw hex values. Light and dark themes via
  a `data-theme` attribute set from the saved preference (system for now).
- Layers and dependency direction: `styles/` (tokens) and `model/` (pure
  view models: line assignment, filters, paint, status, remembered scopes, times,
  markdown links, grid movement) → `components/` (primitives and shared
  patterns: Button, Icon, LineBullet, Pill, StateNote, Tabs, SidePanel,
  SegmentedControl, fields and form fields, Dialog, Toast, Markdown,
  RouteBar, EmptyState, RunState, PlanRoute, QuestionCard) → `features/<feature>/` (board,
  card, editing, agents, workstreams, table, projects, filters, archive; later
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
- Visual direction (D18, D22, 2026-10-06): **Metro Pop** in light (a
  strong raspberry band and accent on a clean white ground, cool light-grey
  column wells, white cards; never blue, never cream) and **Night Service**
  in dark (black band,
  neutral charcoal surfaces with no navy, lime primary action). Both follow
  one layout and content contract, `docs/dev/specs/ui-layout.md`: a theme
  changes tokens only, never structure. Colour roles are fixed there (§7):
  line colours mean workstreams (bullet, card stripe, route); paint shows
  the Colour by value as a header tint plus a named tag; status pills
  (neutral, amber, blue, green) and the blocked pill always carry an icon
  and words; Needs you is the one attention plate; dashes mean suspended
  service, a missing station or needs repair; the Working dot is the only
  looping motion and stops under reduced motion. Archivo Variable,
  self-hosted, tabular numerals, sentence case; rounded controls, cards and
  wells. `tokens.test.ts` holds every text pair to WCAG AA in both palettes.
  The approved references live in `.impeccable/mocks/metro-theme-rollout/`;
  `DESIGN.md` records the built system. Icons are an authored SVG set in
  `components/Icon.tsx`.
- Validation: Vitest for view-models and components; Playwright screenshots
  at 1280 and 1920 in light and dark with axe-core for visible changes.
