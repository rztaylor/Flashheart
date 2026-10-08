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
- Browser: `scripts/e2e.sh` builds the binary and runs Playwright with
  `PLAYWRIGHT_BROWSERS_PATH=$PWD/.cache/ms-playwright` (pinned Chromium is
  downloaded once) for lifecycle, screenshots (1280/1920, light/dark, in
  `.cache/playwright-screenshots/`) and axe-core; required for visible UI
  changes. It drives the real binary with `PATH=/nonexistent` so the
  manual-URL path opens the page, against a copy of `testdata/boards/sample`.
- CI runs `scripts/check.sh` on ubuntu-latest and macos-latest and
  `scripts/e2e.sh` on ubuntu-latest for every pull request and push to
  `main`.
- Tests never touch the real board root or real agent configuration: use
  temporary directories, `testdata/boards/` copies, and fixture config files.
- Hook adapters are tested against recorded, scrubbed payloads in
  `testdata/hooks/<agent>/<event>/` with expected events beside each; the
  agent's `MANIFEST.md` says which are recorded. Re-record with
  `scripts/record-claude-hooks.sh` when an agent changes.
- Hook latency: `scripts/hook-bench.sh` (developer tool, not in CI) fails
  when p95 over 1,000 warm invocations exceeds 50 ms.
- The Playwright Agents spec creates runs by running `flashheart hook claude`
  against the sandbox (`frontend/e2e/agent-runs.mjs`).
- Run-state and blocking logic are table tests with a fake clock.
- MCP tools are tested through the Go SDK's in-memory transport; the
  protocol end-to-end smoke (agent-protocol §13) is `TestProtocolSmoke` in
  `internal/cli`, which drives the real `hook` and `mcp` commands and runs
  with `go test`.
- Setup is tested against `testdata/setup/claude/` in a temporary home with
  a fake `claude` CLI; tests never touch the real `~/.claude`. `doctor` is
  tested the same way, with fixture homes and board roots (a missing hook, a
  moved binary, a hook-errors log).
- The Runs tab tree is tested from the golden hook payloads' events
  (`TestTicketRunsCarryTheSubagentTree` in `internal/api`), and in the
  browser from subagents the Agents e2e starts through the real hook.
- The Agents e2e asks a question through the real `flashheart mcp` and
  answers it in the browser (`frontend/e2e/agent-runs.mjs`), and drives a
  hookless Codex session through it (`connectMCP`) to its answer and end.
- Runs the MCP server starts (D30) are tested over the in-memory transport
  with several client connections (`env.dial`), and over stdio by
  `TestHooklessSessionOverStdio` in `internal/cli`.
- Concurrency: multi-process write tests for locks and preconditions.
  `store.LockWait` is a variable so a non-parallel test can shorten it (the
  MCP `busy` contract test); restore it before returning.
- MCP scale (NFR-1, developer tool, not in CI): `go test ./internal/mcpserver
  -run '^$' -bench ToolsAtScale -benchtime 5x` times tools on 50 projects of
  1,000 tickets (about 0.2 s a call on an M-series Mac, 2026-10-07).
