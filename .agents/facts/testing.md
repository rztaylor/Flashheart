# Testing facts

- Full local validation: `scripts/check.sh`. Until `foundation` lands it skips
  the Go or frontend half when `go.mod` or `frontend/package.json` is absent;
  `foundation` makes it strict.
- Backend: `GOCACHE=$PWD/.cache/go-build go test ./...`, the same with
  `-race`, `go vet ./...`, and `gofmt -l cmd internal` must be empty.
- Frontend: `npm --prefix frontend run lint`, `npm --prefix frontend test`,
  `npm --prefix frontend run build`.
- Browser: `PLAYWRIGHT_BROWSERS_PATH=$PWD/.cache/ms-playwright npm --prefix
  frontend run test:e2e` for lifecycle, screenshots (1280/1920, light/dark)
  and axe-core; required for visible UI changes.
- Tests never touch the real board root or real agent configuration: use
  temporary directories, `testdata/boards/` copies, and fixture config files.
- Hook adapters are tested against recorded, scrubbed payloads in
  `testdata/hooks/<agent>/<event>/`; re-record when an agent changes.
- Run-state and blocking logic are table tests with a fake clock.
- MCP tools are tested through the Go SDK's in-memory transport.
- Concurrency: multi-process write tests for locks and preconditions.
