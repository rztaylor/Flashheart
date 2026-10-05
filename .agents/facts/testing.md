# Testing facts

- Full local validation: `scripts/check.sh` (strict: frontend `npm ci`, lint,
  unit tests and build, then gofmt, vet, Go tests, `-race`, binary build). Go
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
  `testdata/hooks/<agent>/<event>/`; re-record when an agent changes.
- Run-state and blocking logic are table tests with a fake clock.
- MCP tools are tested through the Go SDK's in-memory transport.
- Concurrency: multi-process write tests for locks and preconditions.
