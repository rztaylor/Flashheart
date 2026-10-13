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
  row there. Attribution precedence is tested apart from the tools
  (`TestAttributionPrecedence`).
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
  with `go test`.
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
