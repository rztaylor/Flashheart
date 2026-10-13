# Testing facts

- Full local validation: `scripts/check.sh` (strict: frontend `npm ci`, lint,
  unit tests and build, then gofmt, vet, Go tests, `-race`, binary build).
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
  `useRevision`). `src/test/dom-setup.ts` unmounts after each test.
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
  rather than redefining them.
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
- CI runs `scripts/check.sh` on ubuntu-latest and macos-latest and
  `scripts/e2e.sh` then `scripts/e2e-visual.sh` on ubuntu-latest for every
  pull request and push to `main`.
- Tests never touch the real board root or real agent configuration: use
  temporary directories, `testdata/boards/` copies, and fixture config files.
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
- Ticket-editing rules (FH-85) are table tests where they are defined:
  the field change-set, the blocked-start gate and `NoteLine` in `board`,
  the write in `store` (`TestEditTicket`). `internal/archtest` keeps them
  single: the note bullet is spelled only in `board.NoteLine`, and `api`
  and `mcpserver` never check fields, set titles or change workstreams
  themselves.
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
  text, dropped files, keys left to the page). Pasted HTML (FH-142, D66)
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
- The tool list has three hand-kept copies: the registrations, the
  agent-protocol §7.2 table and the skill. `TestSkillAndSpecNameExactlyTheRegisteredTools`
  (`internal/mcpserver`) lists tools over the in-memory transport and fails
  when either copy misses a tool or names one not registered; a word the
  skill puts in backticks must be a tool, a tool input, a ticket value or
  in its short list. `update_ticket`'s description lists every field it
  sets (`TestUpdateTicketDescribesEverySettableField`).
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
