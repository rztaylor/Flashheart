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
  markdown links, grid movement, section edits, lightbox zoom and pan,
  `messageOf` for a failure's words) →
  `components/` (primitives and shared
  patterns: Button, Icon, LineBullet, Pill, StateNote, Tabs, SidePanel,
  SegmentedControl, fields and form fields, Dialog, Toast, Markdown,
  Table, TailText, BranchLabel, EmptyState, RunState, NeedsYou,
  TicketMarks (a ticket's type icon and three-bar priority marker, alone on
  cards and before their words in tags elsewhere, FH-154),
  QuestionCard, Aside, Popover,
  FilterChip, ChipInput, TicketLink, LoadError, StaleNote, and `useKeepFocus`, the
  one owner of keeping keyboard focus on a moved or removed list item,
  ui-layout.md §8; KeyBadge draws a project's image in place of its key
  when it has one, FH-151) → `features/<feature>/` (board,
  card, editing, editor, overview, workstreams, table, projects (the
  rail, and `ProjectImageMenu`, the page header's badge as the control
  that chooses, replaces or removes the project's image through
  `api/files`), filters, archive; later
  settings) → `app/`
  (shell, hash routing, composition). Shared layers (`styles/`, `model/`,
  `components/`, `api/`, `state/`, `lifecycle/`, `test/`) never import
  features or the shell, `model/` never imports components, and a feature
  never imports the shell; features may use each other. Checked by
  `src/layering.test.ts` (FH-106), which reads every source's relative
  imports, so `npm test` and `scripts/check.sh` fail on a wrong one. A
  type two layers need lives in the lower one (`Scope` in
  `model/scopes`, `Density` in `api/preferences`).
- `src/lifecycle/` owns the single Singleserve session (`connect` from
  `/_singleserve/client.js`), heartbeat, backend-lost and terminal states.
  `src/api/` owns typed requests via `session.fetch` only. `src/state/` owns
  loading and freshness: `useRevision` long-polls `/api/changes` and
  `useResource` keeps the last good value and reloads when the revision
  moves; `usePreferences` saves UI preferences through the backend as a
  patch of what changed (`PATCH /api/preferences`) and reads them again
  as the revision moves when nothing of its own is waiting (FH-173, D80;
  a refreshed copy restores the open scope's filters in
  `app/useScopeMemory`);
  `leaveGuard` is the one guard on leaving the page with unsaved work.
- Load states (ui-layout.md §8, FH-103) are one contract: a view whose
  load fails shows `components/LoadError` (the error in words, an alert,
  and Try again calling the resource's `reload`); a view whose reload
  fails keeps its data and shows `components/StaleNote`; while loading,
  each view shows the one skeleton its feature exports (`BoardSkeleton`,
  `TableSkeleton`, `OverviewSkeleton`, `WorkstreamsSkeleton`,
  `ArchiveSkeleton`, `TicketSkeleton`; the workstream page its own),
  marked `data-skeleton`.
- Editing: every edit from the panel carries the ticket's content hash
  through one helper, `features/editing/precondition` (`saveWithBase`:
  saved, conflict or failed). A draft (the panel's Edit tab, each
  in-place item on the full page) is `features/editing/useTicketDraft`:
  its base hash (moved on to a newer version that left the draft's own
  part unchanged, and a save that meets such a version saved again on
  it), adopting the saved version, and a 409 as a pending
  choice for the conflict dialog; a criterion or review-step tick reloads
  with a toast on a 409. The form's fields and changes are
  `model/ticketForm`; a section's text, its comparison and the criteria
  list are `model/sectionEdit`. Moves from the board set only `status`.
  In-place editing on the full page (EDIT-11, FH-66) is
  `features/editing/InPlace`: `InPlaceEditing` holds the one open item
  and the unsaved-changes prompt, `useInPlaceItem` an item (pencil, Add,
  double-click, focus return) and `InPlaceEditor` one open item's draft
  with Save, Cancel, the shortcuts, the inline error and the conflict
  dialog; the card feature supplies each item's control
  (`PageEdits`, `PropertyEdits`); a section edits in the formatted
  editor, `features/editor`'s `MarkdownField` (EDIT-12), through
  `features/editing/InPlaceSection`, the one section editor. A
  workstream's page (EDIT-13, FH-139) is
  `features/workstreams/WorkstreamPage`, its sections the same
  `InPlaceEditing` items and `InPlaceSection` (with `kind="workstream"`
  for the conflict dialog's words) over `model/workstreamSections`, saved
  by `saveWorkstreamSection`. Its name is `workstreams/WorkstreamName`,
  an `InPlaceEditor` saved by `nameWorkstream` (`slugFromTitle`: the
  server makes the slug, FH-167, D78); a new slug moves the page with
  `useRoute`'s `replace` and rewrites the loaded preferences
  (`app/scopeMemory`'s `renameWorkstreamIn`, as the server did, FH-170).
  A 404 after the page loaded looks for what the workstream became
  (`workstreams/successor`: same name, or all its tickets) and follows it
  the same way; else the page's `EmptyState` (as its h1) says it was
  renamed or removed (FH-174, D80).
  A control
  marked `data-autofocus` takes an open item's focus, and a key a
  control has handled (`defaultPrevented`) is not the item's. A person's comment in Activity is an
  item too, saved on the activity file's hash (`ActivityTimeline`); a
  comment being written in `CommentBox` is a held draft
  (`useHeldDraft`), asked about on leaving but not when opening an item
  (FH-125). The box's Tell a session now (FH-127) offers the runs the
  ticket detail lists (`tellSessions`), grouped and worded by
  `model/tell`; a comment's delivery line and its edit lock once
  delivered are `ActivityTimeline`'s, and its Delete control, with its
  confirmation and refusal reasons (`model/comments`' `deleteRefusal`),
  is `card/CommentDelete` (FH-140). Timeline and box are one shared part, `TicketComments`, on
  the page's Activity and the card panel's Comments tab, which has its
  own `InPlaceEditing` and stays mounted across tabs; closing the panel
  or opening another ticket waits on the leave guard (FH-128). The
  panel's Edit tab stays mounted too and is a held draft whose
  `returnTo` shows the tab and refocuses it on Keep editing (FH-138).
  Leaving
  the page with unsaved changes
  goes through the one leave guard, `state/leaveGuard`, which the router
  (`app/route`) asks before every navigation and a link or Back that
  changed the hash; the browser's `beforeunload` prompt covers reloads.
  A save waiting for its answer is not unsaved (D70, FH-156): an open
  item, the Edit tab and the comment box stop holding the guard while
  busy, and `useTicketDraft` and the box hold only the reload prompt
  (`guardUnloading`) until the answer. A failure or conflict that
  arrives after the draft unmounted goes to `features/editing/lateSaves`
  (`LateSavesProvider` in the shell, the tab's memory only): its toast's
  **Back to the edit** asks the shell to go back to the `EditPlace`
  (ticket page, panel tab, workstream page), whose `InPlaceEditing`
  (`place`) reopens the item, which takes the held draft as it opens;
  held drafts take theirs with `useLateDraft`.
  Native `<dialog>` for decisions (blocked move, conflict, new ticket,
  new workstream; `components/Dialog` opens with focus on the field
  marked `data-autofocus`, which `TextInput` and `TextArea` set for
  `autoFocus`); a polite toast with Undo for results. Only the latest move offers Undo:
  the toast clears as a move starts, and a save that returns after a later
  move started is announced without Undo (FH-115). Drag and drop uses
  `@dnd-kit` pointer sensors; the keyboard alternative is Shift with an
  arrow, and the panel's Move to menu.
- Tickets are named by id (`FH-42`) everywhere: cards, panel, table,
  search and the `?t=<id>` route parameter. Every shown id is a
  `components/TicketLink` to the ticket's full page `#/ticket/<id>`
  (`features/card/TicketPage`, CARD-7) in a new tab, the same tab inside
  `TicketLinksInPlace` (the page). Bare ids of known project keys in
  markdown and in plain text (`LinkedText`) link there too
  (`model/markdown.ts`, KEY-3, D30). A link never sits inside a button:
  cards and stations draw under one covering button with the id link above
  it (ui-layout.md "Ticket ids").
- Prohibited: raw `fetch` to the backend, credentials in JavaScript, Web
  Storage for anything, permissive CORS, routes under `/_singleserve/`, raw
  HTML in markdown.
- Drag and drop: `@dnd-kit` with keyboard sensors; every drag has a menu
  alternative. Markdown: `react-markdown` + `remark-gfm` renders it;
  CodeMirror 6 edits it (`features/editor`, D61, D62): the markdown text
  itself, never re-serialised, with a Flashheart live preview, slash menu
  and selection toolbar. It loads on the first edit (a dynamic import, its
  own chunk) and mounts in a shadow root, where its run-time styles are a
  constructed style sheet the CSP (`style-src 'self'`) allows; Tailwind
  does not reach inside, so it is themed with `EditorView.theme` over the
  token custom properties. Paste (`editor/paste`, D68): files first, then
  plain text after Ctrl or Cmd with Shift and V (the key is observed, as
  the paste event does not carry it), then copied HTML as markdown
  through `editor/htmlToMarkdown` (our own walk over an inert
  `DOMParser` document: allow-listed link schemes, remote images as
  links, headings from level 3, never raw HTML), else plain text. Files
  (REV-7, FH-18): a pasted screenshot, dropped or chosen files go into
  the ticket through `MarkdownField`'s `upload` (`editor/uploads`: a
  placeholder decoration per file, then its reference typed in); the panel's Attachments tab and
  the page's Files take them through `card/AttachmentDrop`; both use
  `components/FileDrop` and `model/uploads`, and `app/fileDrops` keeps a
  file dropped elsewhere from opening in the tab.
- Desktop first (1280 px and up); narrow widths show one column with a column
  picker, and widening past 48rem puts the board's column strip back at its
  first column (`features/board/useStartOnWiden`, FH-155). Healthy
  connection state is visually quiet.
- The app fits the window and the document never scrolls: each view scrolls
  in its own column. Every scrolling column is `relative`, so absolutely
  positioned content inside it, `sr-only` labels included, stays inside it
  rather than resolving against `<body>` and stretching the document into a
  second, empty scrollbar (FH-178; `board.spec.mjs` checks every view).
- Visual direction (D18, D22, 2026-10-06): **Metro Pop** in light (a
  strong raspberry band and accent on a clean white ground, cool light-grey
  column wells, white cards; never blue, never cream) and **Night Service**
  in dark (black band,
  neutral charcoal surfaces with no navy, lime primary action). Both follow
  one layout and content contract, `docs/dev/specs/ui-layout.md`: a theme
  changes tokens only, never structure. Colour roles are fixed there (§7):
  line colours mean workstreams (bullet, card stripe, route); paint shows
  the Colour by value as a whole-card tint, named by its colour chip and
  the card's name; status pills
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
  Keys, focus and state over time are tested in happy-dom with Testing
  Library (`*.dom.test.tsx`, D42); a visible change updates the stable
  screenshot baselines (`scripts/e2e-visual.sh --update-snapshots`).
