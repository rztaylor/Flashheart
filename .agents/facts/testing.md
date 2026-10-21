# Testing facts

- Full local validation: `scripts/check.sh` (strict: boundary-note sizes,
  `scripts/check-boundaries.sh`; frontend `npm ci`, lint, unit tests and
  build, then gofmt, vet, Go tests, `-race`, binary build).
  The NFR-1 timing test (`TestIndexesFiveThousandTicketsQuickly`, 5,000
  tickets in under 1 s) is skipped in the parallel passes and run alone
  afterwards, so CPU contention on small CI runners cannot fail it. Go
  tests need the compiled frontend, so the frontend build runs first.
- Backend: `GOCACHE=$PWD/.cache/go-build go test ./...`, the same with
  `-race`, `go vet ./...`, and `gofmt -l cmd internal` must be empty.
- Frontend: `npm --prefix frontend run lint`, `npm --prefix frontend test`,
  `npm --prefix frontend run build`.
- Frontend unit tests run as two Vitest projects (D42): `unit` in node
  (view models, static markup with `renderToStaticMarkup`) and `dom` in
  happy-dom for files named `*.dom.test.ts(x)`, which use Testing Library
  (`render`, `renderHook`, `user-event`) to press keys, tab, click and
  check `document.activeElement` (Popover, Dialog, Tabs, `useResource`,
  `useRevision`). `src/test/dom-setup.ts` unmounts after each test and
  fires happy-dom's `selectionchange` as a queued task, as browsers do
  (happy-dom fires it inside `Selection.collapse`, which re-enters
  CodeMirror's update from `@codemirror/view` 6.43.14, FH-184).
  A hook test drives time with Vitest's fake timers and a fake fetcher or
  loader whose promises the test settles.
- Browser: `scripts/e2e.sh` builds the binary and runs Playwright with
  `PLAYWRIGHT_BROWSERS_PATH=$PWD/.cache/ms-playwright` (pinned Chromium is
  downloaded once) for lifecycle, screenshots (1280/1920, light/dark, in
  `.cache/playwright-screenshots/`) and axe-core; required for visible UI
  changes. It drives the real binary with `PATH=/nonexistent` so the
  manual-URL path opens the page, against a copy of `testdata/boards/sample`.
  Specs share `open`, `card`, `column`, `expectConnected`,
  `expectNoAxeViolations` and `shot` from `frontend/e2e/support.mjs`
  rather than redefining them. After a save, a step waits for what the
  page shows once the save's own answer is back (its toast, the editor
  closing, Save disabled again) before going on: the file and the live
  update show the change first, and a late answer lands in the next step
  (FH-152). The pencil focused, the comment box empty, and
  `saveChanges(panel)` (support.mjs) disabled, which waits past
  *Saving…*, are those answers (FH-156). To find steps that skip it, a
  local, uncommitted context route that fetches every non-GET `/api`
  request and fulfils it 0 to 1.5 s later reproduces a slow machine. A
  step that needs the page not to have heard of an outside write writes
  the file in a `page.route` handler for the save request itself, before
  continuing it; holding back `/api/changes` cannot promise that, since a
  long poll already waiting still answers (FH-164).
- A save that fails after its item was left (FH-156, D70): the page, the
  Edit tab and the comment box in `LateSave.dom.test.tsx`, a harness
  standing for the shell over `LateSavesProvider`, with writes held until
  the test answers them (no prompt while in flight, the toast, Back to
  the edit with the text, the conflict, reopening by hand, the reload
  prompt); the leave guard's reload-only hold in `leaveGuard.dom.test.ts`;
  in the browser, `frontend/e2e/in-place.spec.mjs` holds a save's reply
  with `page.route`, leaves, fails it, and uses the toast's action, with
  axe and screenshots of the toast.
- Visual regression (D42): `scripts/e2e-visual.sh` compares the stable
  screenshot set (`frontend/e2e/visual.spec.mjs`: board, panel, table,
  workstreams, Overview and ticket page over the demo board, at a fixed
  clock and file times) with the baselines in
  `frontend/e2e/__screenshots__/`, in the Playwright Docker image matching
  the pinned `@playwright/test`, on amd64; `scripts/e2e.sh` skips the set.
  After an intended visual change, run
  `scripts/e2e-visual.sh --update-snapshots`, look at the changed PNGs and
  commit them with the change. Diffs of a failure are in
  `.cache/playwright-results` (a CI artifact).
- README screenshots (FH-180, developer tool, not in CI or
  `scripts/e2e.sh`): `scripts/readme-screenshots.sh` (`--no-build` reuses
  `build/flashheart`) runs `frontend/e2e/readme/playwright.config.mjs`,
  whose spec (`screenshots.shots.mjs`, not a `*.spec.mjs`, so the suite
  never picks it up) writes the invented showcase board
  (`showcase-board.mjs`: three projects, every column, six workstreams,
  fixed dates and file times relative to now, its own assertions in
  `checkShowcaseBoard`) into a temporary root, records sessions through
  the real hooks and MCP server (`agent-runs.mjs`), serves it and writes
  1440 px, scale 2 PNGs to `docs/images/`, then pngquant shrinks them
  when installed. Look at every image before committing it.
- CI runs `scripts/check.sh` on ubuntu-latest and macos-latest and
  `scripts/e2e.sh` then `scripts/e2e-visual.sh` on ubuntu-latest for every
  pull request and push to `main`, and as the first stage of the release
  workflow (`release.yml` calls `ci.yml`, D86).
- Release tooling (FH-179, D86): `scripts/release`'s table tests cover
  the changelog's version and notes (a released section, only
  Unreleased, malformed headings, empty and over-long notes) and parse
  the real `CHANGELOG.md`; packaging is tested with a fake build (names,
  entries, modes, owners, `SHA256SUMS`, stale archives replaced) and by
  running `scripts/package.sh --skip-frontend` for the host platform and
  the packaged binary's `version`. A workflow change is linted with
  `actionlint` where it is installed (not part of `scripts/check.sh`) and
  its triggers and permissions are reviewed by hand.
- Tests never touch the real board root or real agent configuration: use
  temporary directories, `testdata/boards/` copies, and fixture config files.
- The API contract (FH-97, D79): `internal/api`'s `TestAPIContract` plays
  one scripted session on a copy of `testdata/boards/sample` (every route
  but the two in `routesWithoutFixture`, in order) and keeps each
  response in `testdata/api/<name>.json`, with random activity ids and
  hashes (numbered within each file), the root, file times and board
  revisions replaced by stable placeholders (the sample's file times
  are pinned first, since projects list newest first and a Linux clock
  tick can tie them, FH-176), so a new step changes only
  its own fixture; a merge conflict there is settled by rerunning the
  update after the merge. It
  fails when a fixture is stale, when a registered route has none, or when
  a response type carries a `board`, `config`, `runs` or `events` type
  (`domainPackages`; FH-175 gave the last board and config pieces their
  own answer shapes, with the JSON unchanged). `TestVocabulary`
  keeps `testdata/vocabulary.json`; `TestErrorCodes` holds handlers to
  `api.ErrorCodes`. After a deliberate change, rewrite them with
  `go test ./internal/api -run 'TestAPIContract|TestVocabulary' -update`
  and commit the diff. `frontend/src/api/contract.test.ts` then runs each
  fixture through its guard: it must pass, and fail with any field
  removed (unless listed as optional, the server's `omitempty`) or given
  another JSON type; a new fixture needs an entry there.
  `frontend/src/api/vocabulary.test.ts` matches the frontend's lists to
  the vocabulary file. A new endpoint adds a step to `runContract`, a
  guard, and an entry in the frontend's contract table.
- Hook adapters are tested against recorded, scrubbed payloads in
  `testdata/hooks/<agent>/<event>/` with expected events beside each; the
  agent's `MANIFEST.md` says which are recorded. Re-record with
  `scripts/record-claude-hooks.sh` or `scripts/record-codex-hooks.sh` when
  an agent changes; the Codex recorder runs with a scratch `CODEX_HOME`
  (Codex saves project trust to its config even with
  `--ignore-user-config`), so it never changes `~/.codex`.
- Codex hook trust (the key and hash doctor recomputes, agent-protocol
  §5.4) is tested against Codex's own keys and hashes in
  `testdata/setup/codex/trusted/`; re-record them with
  `scripts/record-codex-hook-trust.sh` (scratch `CODEX_HOME`, drives
  `codex app-server`, no login or model) when Codex updates its hooks.
- Hook latency: `scripts/hook-bench.sh` (developer tool, not in CI) fails
  when p95 over 1,000 warm invocations exceeds 50 ms and prints how many
  events were recorded. `-agent` picks the payloads
  (`testdata/hooks/<agent>`); `-scenario` is `mix` (default), `tools`
  (tool results alone, the throttled path, FH-55; also `-tools`),
  `session-start` (the recovery note) or `stop` (turn ends that handoff
  enforcement blocks); `-seed-events N` seeds N events a day across the
  run window first. HOOK-1 evidence is `session-start` and `stop` at
  `-seed-events 50000` (FH-95): p95 about 36 ms and 30 ms on an M4 Max,
  2026-10-12. The stop scenario's session is already due, so it does not
  run the git worktree check.
- Hook folds read only the sessions they ask about; their equivalence with
  a full fold is tested on random mixed logs
  (`TestKeepFoldsTheSessionsAsTheFullFoldDoes`,
  `TestLatestFindsThePreviousSessionAsTheFullFoldDoes`) and the selective
  read against `Read` (`TestReadSelectedGivesWhatReadGivesThatKeepAccepts`).
- Activity throttling (FH-55) is tested by replaying recorded payloads
  through the Claude adapter (200 tool results in ten minutes) and with the
  fake adapter's scripts in `internal/hooks`; the run-state and no-handoff
  tables run on both encodings of tool use (`tool.used` and `activity`).
- The Playwright runs spec (`frontend/e2e/runs.spec.mjs`: the Overview,
  live cards, the Runs tab and questions) creates runs by running
  `flashheart hook claude`
  against the sandbox (`frontend/e2e/agent-runs.mjs`).
- Load states (ui-layout.md §8, FH-103): views are rendered statically with
  `useResource` mocked for their error, stale and loading states; in the
  browser, `frontend/e2e/states.spec.mjs` fails requests (500) or holds
  them with `page.route` to check each view's Try again, last-good-copy
  note and skeleton, with screenshots named `state-error-*`,
  `state-stale-*` and `state-loading-*`.
- Run-state and blocking logic are table tests with a fake clock.
- An agent's rewrite (FH-69, D75): which sections, criteria operations
  and what a rewrite replaced are table tests in `board`
  (`rewrite_test.go`, with a corpus check that criteria edits leave the
  rest of real FH tickets byte for byte); the kept text, its index entry,
  the history and restoring from it in `store` (`rewrite_test.go`); the
  inputs, refusals, `claimed` and `conflict` in `mcpserver`, the
  conflict through the server's `betweenReadAndWrite` seam, which changes
  the ticket between `update_ticket`'s read and its write.
- Layering (FH-106) is tested, not reviewed: `internal/archtest`'s
  `TestLeanPackagesDoNotLinkTheWebServer` walks `go list -deps` of the
  hook, MCP, await, setup and doctor packages and of `cli` (FH-169: it
  fails when any of them reaches `index`, `api`, `app`, `webui` or
  `net/http` outside the MCP SDK allowance), and the frontend's
  `src/layering.test.ts` (Vitest, over `import.meta.glob` raw sources)
  checks every relative import against the layer rules in
  `frontend-ui.md`. Both run in `scripts/check.sh` through `go test` and
  `npm test`.
- Ticket-editing rules (FH-85) are table tests where they are defined:
  the field change-set, the blocked-start gate and `NoteLine` in `board`,
  the write in `store` (`TestEditTicket`). `internal/archtest` keeps them
  single: the note bullet is spelled only in `board.NoteLine`, and `api`
  and `mcpserver` never check fields, set titles or change workstreams
  themselves.
- Workstream edits (FH-46): the change-set, its byte-for-byte edits over
  `testdata/workstreams/` and the cycle rule are table tests in `board`
  (`workstreamedit_test.go`); the one write with its members, refusals
  that write nothing, the hash, the lock (`ErrBusy`, not parallel) and a
  ticket added after a workstream dependency was set in `store`
  (`workstreams_test.go`); `update_workstream`, `create_ticket` with
  `project` and the blocked reasons `list_tickets`, `get_ticket` and
  `board_context` show in `mcpserver` (`workstreams_test.go`, with
  `TestWriteToolPolicies`' row). `internal/archtest` keeps `mcpserver`
  from editing a workstream file itself.
- Workstream slug renames (FH-158): every reference rewritten (members,
  other workstreams' and live and archived tickets' dependencies) with
  edits in the same write, and refusals (taken, invalid, stale) that
  write nothing, in `store` (`workstreamrename_test.go`); `rename_to` and
  its events in `mcpserver` (`workstreams_test.go`); the board's PATCH in
  `api` (`TestPatchWorkstreamRenamesIt`) and its authentication in `app`.
  Renaming from the page's name (FH-167, D78): the slug made from the
  name, kept when it is the same or made unique, and a taken name in
  words of names in `api` (`TestPatchWorkstreamNamesItsSlugFromTheTitle`)
  and `store` (`TestWorkstreamSlugFor`); the saved preferences a rename
  rewrites, the ones it leaves, and a broken `config.yaml` that does not
  stop it in `store` (`workstreamprefs_test.go`), `config`
  (`TestUIRenameWorkstream`) and `api`
  (`TestPatchWorkstreamRenameKeepsSavedPreferences`), the page's copy in
  `app/scopeMemory.test.ts`; the name item in
  `WorkstreamPage.dom.test.tsx`, the route's replace in
  `route.dom.test.ts`; in the browser, `workstream-page.spec.mjs`
  renames a line from its route card's page (the taken name, every
  reference, the saved filter through a later preference save, Back and
  Forward, both themes, 390 px, axe).
  Renames made outside the tab (FH-173, FH-174, D80): the preference
  patch in `api` (`TestPreferencesPatchChangesOnlyWhatItNames`, and a
  stale tab's theme after a rename in
  `TestPatchWorkstreamRenameKeepsSavedPreferences`); the saver's patches
  in `state/usePreferences.test.ts` and the refresh that never loses a
  change in `usePreferences.dom.test.ts`; the open filters restored in
  `app/useScopeMemory.dom.test.ts`; the successor rule in
  `workstreams/successor.test.ts` and the page that follows or says it is
  gone, through the real `useResource`, in
  `WorkstreamPage.gone.dom.test.tsx`; in the browser,
  `workstream-page.spec.mjs` renames through the real `flashheart mcp`
  while the board is open (the saved filter through a theme and a filter
  change, a change made just before kept) and while the page is open
  (followed, then removed: both themes, 390 px, axe).
- In-place edits (FH-66) leave the rest of a real ticket byte for byte:
  `board` replays field and section edits over `testdata/tickets/`
  (read-only copies of FH tickets, also with CRLF line endings) and
  checks that only the field's line or the section's text changed and that
  an edit with the current value writes nothing; `api` checks the
  section write's precondition, no-op and event. The page's interaction is
  `TicketPageEdit.dom.test.tsx`; in the browser,
  `frontend/e2e/in-place.spec.mjs` saves, cancels, conflicts (writing the
  file during an edit) and fails a save with `page.route`.
- The formatted editor (FH-16) is tested in `frontend/src/features/editor`:
  the live preview's decorations per GFM construct, the commands and the
  slash menu over `EditorState` in node; `corpus.test.ts` opens every
  editable section of `testdata/tickets/` and a GFM sample, checks that
  it comes back exactly (untouched, and through a Formatted/Markdown
  switch) and that a word, bold, tick or added item changes only that
  line; `MarkdownEditor.dom.test.tsx` mounts it in happy-dom (checkbox,
  Alt+Enter, the view switch, pasting a web page as markdown or as plain
  text, dropped files, keys left to the page, and an edit reaching the
  page's state before the next key, outside `act`, as the browser
  delivers them). Pasted HTML (FH-142, D68)
  is a table of realistic copies in `htmlToMarkdown.dom.test.ts` (unsafe
  schemes, remote images, nested lists, code, Google Docs, a code
  editor's lines, never raw HTML; happy-dom is told not to load frames
  or files, which a browser's `DOMParser` document never does), and the
  plain-text key behind a key binding in `paste.dom.test.ts`. Layout and
  real focus in its shadow root (the slash menu, the selection toolbar)
  are `frontend/e2e/editor.spec.mjs`, which also checks saving over an
  agent's checkpoint, the conflict, the remembered view, images, pasting
  a web page (as markdown, with nothing loaded or run, and as plain text
  after the key), that no request leaves the board's origin and no CSP
  violation.
  `TicketPageEdit.dom.test.tsx` stands a text area in for the editor.
- A workstream's page (FH-139): the section rule over copies of the real
  FH workstreams in `testdata/workstreams/` (also with CRLF) in `board`
  (`TestWorkstreamSectionChangeLeavesTheRestOfARealWorkstream`: an
  untouched save writes nothing, an edit changes only that section's
  text); the read and section endpoints (precondition, no-op, conflict
  words, a real workstream copy) in `api` (`workstream_test.go`); the
  page's items in `WorkstreamPage.dom.test.tsx` (the editor stood in, as
  on the ticket page); in the browser, `frontend/e2e/workstream-page.spec.mjs`
  opens it from a route card, saves, writes nothing untouched, uses the
  slash menu, toolbar and remembered view, saves over a change elsewhere
  (heard of and not yet heard of), conflicts on the same section, works by
  keyboard and passes axe in both themes at 1440 and 390 px.
- Workstreams made and filled on the board (FH-17): the create
  endpoint (board-format template, unique slug, refusals in words that
  write nothing, read-only) in `api` (`TestCreateWorkstreamWritesTheTemplate`),
  its authentication in `app`; the membership write (both files, stale
  hash writes nothing) in `api` (`TestPatchWorkstreamKeepsTheListsInStep`)
  and `store`; the dialog in `NewWorkstreamDialog.dom.test.tsx`, the
  panel opened ready to edit in `CardPanelEdit.dom.test.tsx`; in the
  browser, `frontend/e2e/workstream-editing.spec.mjs` creates a
  workstream (Enter, the toast, its page, a second whose title makes the
  same slug), a refusal, the dialog in both themes and at 390 px with
  axe, New ticket's panel on its Edit tab with Title focused, and adds
  and removes a ticket through the picker, reading both files.
- Unique workstream titles (FH-163, D76): the comparison is a table test
  in `board` (`TestSameWorkstreamTitle`) and `model/lines.test.ts`
  (`takenWorkstream`, the same rule); the one store check, through
  create, field edit and raw edit, refusals that write nothing, another
  project, and duplicates made earlier that still load and edit, in
  `store` (`workstreamtitle_test.go`); 409 `title_taken` in `api`
  (`TestCreateWorkstreamWritesTheTemplate`); `invalid_input` with the
  fix in `mcpserver` (`TestCreateWorkstream`,
  `TestUpdateWorkstreamRefusesAndWritesNothing`); the field's message,
  nothing sent, and the server's refusal at the field in
  `NewWorkstreamDialog.dom.test.tsx`; in the browser,
  `workstream-editing.spec.mjs` shows a taken title at the field (axe,
  390 px in both themes) and a title written to disk in the POST's
  `page.route` handler refused by the server.
- A project's image (FH-151, D82): the crop, scale, PNG output, EXIF
  orientation and refusals (not an image, a header claiming too many
  pixels, a truncated picture) are table tests in `imagefit`; the
  `image` field, its warnings (missing, not a picture, not a file name),
  the one-transaction write, replacement under another name, refusals
  that write nothing and clearing in `store` (`projectimage_test.go`);
  the endpoints, the SEC-4 headers, the versioned URL, refusals in words,
  warnings instead of errors and that no event is recorded in `api`
  (`projectimage_test.go`, with contract steps `project-image-set`,
  `projects-with-image`, `error-not-an-image`, `project-image-cleared`);
  authentication for PUT, GET and DELETE in `app`. The badge with an
  image and the key in its name are `ProjectRail.dom.test.tsx` and
  `PageHeader.dom.test.tsx`; the menu, its refusal and focus return
  `ProjectImageMenu.dom.test.tsx`; in the browser,
  `frontend/e2e/project-image.spec.mjs` uploads a PNG from the header
  badge, replaces it with a JPEG, reloads, is refused a text file and an
  SVG, gives a second project an image, and removes it, reading
  `project.yaml` and the stored 128×128 file, with axe and screenshots in
  both themes at 1440 and 390 px.
- Files from the board (FH-18, REV-7, SEC-7): sniffing is a table test in
  `board` (`TestSniffAttachmentJudgesTheContent`), the upload's name,
  index entry, streaming limit and refusals in `store`
  (`upload_test.go`), the endpoint (sniffed type, SEC-4 headers, the
  human's event, refusals in words, a body cut off at the limit) in
  `api`, its authentication in `app`; the placeholder, image and link
  references and refusals over a held upload in
  `MarkdownEditor.dom.test.tsx`, the drop area in
  `AttachmentDrop.dom.test.tsx`; in the browser,
  `frontend/e2e/uploads.spec.mjs` pastes a canvas PNG, drops several
  files, an SVG and an oversized one, drops on the Attachments tab, uses
  Attach files by keyboard, saves and reloads, with axe in both themes,
  390 px, and no request off the board's origin or CSP violation.
- The lightbox (FH-146): Fit, limits, steps, zooming around a point and
  pan bounds are `model/zoom.test.ts`; keys, the wheel, dragging and the
  controls are `components/Lightbox.dom.test.tsx`, which stubs the
  stage's size and the image's natural size; in the browser,
  `frontend/e2e/lightbox.spec.mjs` opens it from the page's Evidence and
  the panel's Attachments, reaches 800% and 25%, pans, checks focus
  return, axe in both themes and 390 px.
- The board's column strip after the window widens from a phone's width
  (FH-155): the reset on leaving the narrow layout, nothing while narrow
  and the listener's removal in `features/board/useStartOnWiden.dom.test.ts`
  over a stubbed `matchMedia`; in the browser, `board.spec.mjs` opens a
  Backlog ticket's panel at 1440 px (so the strip overflows), narrows the
  window to 390 px in place, picks Done, and widens it again, in both
  themes.
- Serve's freshness cost is tested by counting board reads through a
  wrapping `index.Source` (`countingSource` in `internal/index` and
  `internal/api`, FH-96): event appends refold with no read, `Current`
  does not read while watching, a board write reads once. The watcher's
  choice of work is a table test (`TestPendingChangesPlanTheLeastWork`),
  not a timing test. Watcher tests start `Watch` through the `watch`
  helper, whose cleanup waits for it to return before the temporary
  directory is removed (FH-110).
- Every MCP write tool's run and claim policy (agent-protocol §7.1) is one
  table test, tool × caller (`TestWriteToolPolicies`); a new tool adds its
  row there, with the history entries a successful call appends.
- A ticket's activity file (D47) is tested where it is defined: the format
  round-trips byte for byte (CRLF, BOM, unknown entries) and history names
  no ticket text in `board`; every store ticket write appends exactly one
  entry, comments refuse a stale hash, and concurrent writers all land in
  `store` (`TestEveryTicketWriteAppendsOneHistoryEntry`); every board write
  in `api` (`TestBoardWritesAppendOneHistoryEntry`); comments need
  Singleserve's authentication in `app`. Attribution precedence is tested apart from the tools
  (`TestAttributionPrecedence`). The comments `get_ticket` and `claim`
  show, and their budgets, are `internal/mcpserver/comments_test.go`;
  posting, quoting, editing, the conflict, the filter and the leave guard
  are `TicketComments.dom.test.tsx` and, in the browser,
  `frontend/e2e/comments.spec.mjs` (FH-125). The comment tests run on
  both the page and the card panel's Comments tab (`describe.each` over a
  surface); the panel's own flow, from posting there to seeing it on the
  full page, is `frontend/e2e/panel-comments.spec.mjs` (FH-128), which
  also drives the Edit tab's unsaved changes; their tab switch, close,
  Escape and another-ticket paths are `CardPanelEdit.dom.test.tsx`
  (FH-138). Change
  requests (FH-126, REV-6): who may move a comment's state and its history in `board`
  (`requests_test.go`), the one-write request and addressing in `store`,
  a ticket's live holder in `index` (`TestHolderIsTheLiveClaimant`),
  delivery to the holder's inbox and the move warning in `api`, the
  `addressed` input, the warning and `board_context` in `mcpserver`
  (with `TestWriteToolPolicies`' row and the tool-list test), the
  recovery note in `protocol` and `TestProtocolSmoke`; the page and the
  dialog are `TicketRequests.dom.test.tsx`, and
  `frontend/e2e/requests.spec.mjs` drives a live session through the real
  hook and `flashheart mcp`.
- Telling a session now (FH-127, D63): a comment's `sent-to` and
  `delivered` and the edit lock in `board` (`delivery_test.go`);
  delivering once, as the comment reads then, and the lock in `store`;
  `Deliver`'s choice of what to hand over in `events` (`tell_test.go`);
  the live claimants in `index`
  (`TestClaimantsAreTheHolderAndItsLiveSubagents`); offering, re-checking
  (`not_live`), the delivery states and the 409 `delivered` in `api`
  (`tell_test.go`); one test per delivery path, each checking the comment
  is marked delivered: the prompt hook (`hooks`, with a later session's
  recovery note not delivering it), `board_context` and a started run's
  tool result (`mcpserver`) and `await`. The box, the picker and the
  delivery line are `TicketTell.dom.test.tsx` on the page and the panel;
  `frontend/e2e/tell.spec.mjs` sends a comment to a session seeded by
  `agent-runs.mjs`, delivers it through the real prompt hook and ends
  the session for the missed case.
- Deleting a comment (FH-140, D66): which comments may go, the entry
  removed byte for byte and the `deleted` history in `board`
  (`deletion_test.go`); the lock, hash, refusals and a delete racing a
  delivery in two processes in `store` (`deletecomment_test.go`); a
  deleted comment or change request left out by `Deliver` and `Withdraw`
  in `events` (`withdraw_test.go`); one test per delivery path that it is
  never handed over (`hooks`, `mcpserver`, `await`, each
  `tell_test.go`); the endpoint, refusals in words, the human's
  `comment.deleted` and the withdrawal in `api`
  (`deletecomment_test.go`), its authentication in `app`. The control,
  confirmation and refusal reasons are `TicketCommentDelete.dom.test.tsx`
  on the page and the panel; `frontend/e2e/delete-comment.spec.mjs`
  deletes from both, reloads, checks the real prompt hook never hands
  over a deleted comment, and shows the refusals, with axe in both themes.
- The tool list has three hand-kept copies: the registrations, the
  agent-protocol §7.2 table and the skill. `TestSkillAndSpecNameExactlyTheRegisteredTools`
  (`internal/mcpserver`) lists tools over the in-memory transport and fails
  when either copy misses a tool or names one not registered; a word the
  skill puts in backticks must be a tool, a tool input, a ticket value or
  in its short list. `update_ticket`'s description lists every field it
  sets (`TestUpdateTicketDescribesEverySettableField`) and its `set`
  schema types each of them and no other, lists as arrays of strings
  that also take one comma-separated string
  (`TestUpdateTicketSetTypesEverySettableField`, FH-171, D83); both
  derive the list from `board.EditableField`'s fields less `status`.
  Setting every field in one call, the comma form, clearing, and values
  of the wrong type refused by the schema are
  `TestUpdateTicketSetTakesListsAndStrings`.
- MCP tools are tested through the Go SDK's in-memory transport; the
  protocol end-to-end smoke (agent-protocol §13) is `TestProtocolSmoke` in
  `internal/cli`, which drives the real `hook` and `mcp` commands and runs
  with `go test`; `TestCrossAgentHandoff` beside it hands a ticket from
  Claude Code to Codex and back, Codex's side replaying
  `testdata/hooks/codex` with its session and working directory swapped.
- Setup's question (FH-129) is a table test in `internal/cli`
  (`TestSetupAsksBeforeApplying`) with the harness's fake stdin and
  `terminal` flag: each answer, end of input, no terminal, `--yes`,
  `--write` and `--dry-run`. `IsTerminal` is tested on a pipe and
  `/dev/null`; a real terminal is checked by hand under a pseudo-terminal.
- Setup is tested against `testdata/setup/claude/` in a temporary home with
  a fake `claude` CLI, and against the `$CODEX_HOME` fixtures in
  `testdata/setup/codex/` (`user/`: the user's own entries; `stale/`: an
  older setup with a moved binary; or empty) copied into a temporary home,
  for diff, write, uninstall and diagnosis; tests never touch the real
  `~/.claude` or `~/.codex`. `doctor` is tested the same way, with fixture
  homes and board roots (a missing hook, a moved binary, a hook-errors log).
  `setup codex` registers exactly the events recorded in
  `testdata/hooks/codex/` less those the adapter ignores
  (`TestCodexHooksAreTheRecordedEvents`); an ignored event's recorded
  payloads must give the adapter nothing to do
  (`TestCodexIgnoredEventsAreIgnoredByTheAdapter`).
- The Runs tab tree is tested from the golden hook payloads' events
  (`TestTicketRunsCarryTheSubagentTree` in `internal/api`), and in the
  browser from subagents the runs e2e starts through the real hook.
- The runs e2e asks questions through the real `flashheart mcp` and
  answers them in the card panel and on the Overview
  (`frontend/e2e/agent-runs.mjs`), and drives a hookless Codex session
  through it (`connectMCP`) to its answer, a branch switch and its end.
- Runs the MCP server starts (D34) are tested over the in-memory transport
  with several client connections (`env.dial`), and over stdio by
  `TestHooklessSessionOverStdio` in `internal/cli`.
- Codex attribution (D45) replays recordings in `internal/mcpserver`
  (`codexruns_test.go`): hook events from
  `testdata/hooks/codex/<Event>/<case>.events.json` go into the log
  (`env.codexHooks`) and calls carry the `_meta` of
  `testdata/hooks/codex/mcp/tools-call-flashheart.json` (`codexMeta`,
  `env.callAs`).
- Concurrency: multi-process write tests for locks and preconditions.
  `store.LockWait` is a variable so a non-parallel test can shorten it (the
  MCP `busy` contract test); restore it before returning.
- MCP scale (NFR-1, developer tool, not in CI): `go test ./internal/mcpserver
  -run '^$' -bench ToolsAtScale -benchtime 5x` times tools on 50 projects of
  1,000 tickets (about 0.2 s a call on an M-series Mac, 2026-10-07).
- Permanent deletes (FH-26, developer tool, not in CI): `go test
  ./internal/store -run '^$' -bench DeleteArchivedLargeArchive -benchtime
  20x` times the preview, the whole delete and the part under the root
  lock with 2,000 archived tickets (about 80, 170 and 90 ms on an M4 Max,
  2026-10-12).
