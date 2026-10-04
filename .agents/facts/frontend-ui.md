# Frontend UI facts

- Framework: React with strict TypeScript, built by Vite into static assets
  embedded in the Go binary. Root `frontend/`, source `frontend/src/`.
- Styling: Tailwind CSS v4. Design tokens (colour, spacing, type, radius,
  focus, motion) are CSS custom properties defined once in
  `src/styles/tokens.css` and exposed to Tailwind through `@theme`; components
  use token-backed utilities, never raw hex values. Light and dark themes via
  a `data-theme` attribute set from the saved preference.
- Layers and dependency direction: `styles/` (tokens) → `components/`
  (primitives and shared patterns: Button, IconButton, Badge, Menu, Dialog,
  Panel, Tabs, Markdown, EmptyState) → `features/<feature>/` (board, agents,
  workstreams, table, card, projects, settings) → `app/` (shell, routing,
  composition). Shared layers never import features or the shell.
- `src/lifecycle/` owns the single Singleserve session (`connect` from
  `/_singleserve/client.js`), heartbeat, backend-lost and terminal states.
  `src/api/` owns typed requests via `session.fetch` only. `src/state/` owns
  the revisioned board snapshot and long-poll loop.
- Prohibited: raw `fetch` to the backend, credentials in JavaScript, Web
  Storage for anything, permissive CORS, routes under `/_singleserve/`, raw
  HTML in markdown.
- Drag and drop: `@dnd-kit` with keyboard sensors; every drag has a menu
  alternative. Markdown: `react-markdown` + `remark-gfm`.
- Desktop first (1280 px and up); narrow widths show one column with a column
  picker. Healthy connection state is visually quiet.
- Current tokens are provisional neutrals. Visual direction: not yet chosen.
  Design with the `impeccable` skill before `board-core` UI work and record
  the approved direction here and in `DESIGN.md`.
- Validation: Vitest for view-models and components; Playwright screenshots
  at 1280 and 1920 in light and dark with axe-core for visible changes.
