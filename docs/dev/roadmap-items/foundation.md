# foundation

Status: **Done**, awaiting review (2026-10-04). Outcomes are in `docs/SPEC.md`
(`LIFE-4`), `docs/dev/decisions.md` (D10, D13), `CHANGELOG.md` and the tests;
delete this brief once the review is approved.

Delivered differences from the scope below: `serve` detaches from the terminal
by default (`--foreground` keeps it attached), which added
`internal/background`; Go 1.26.6 (required by Singleserve v0.2.2) instead of
1.26.5; the Playwright quit-denial step injects the denial response because
nothing can hold a write open yet, while `internal/app` tests drive the real
guard. Browser checks run with `scripts/e2e.sh`, not `scripts/check.sh`.

## Goal

A buildable, testable skeleton that every later item extends: Go module and
package layout, CLI with subcommands, a Singleserve-hosted empty React shell
with the complete lifecycle UI, the frontend toolchain, the validation script,
and shared board fixtures.

## Scope

- `go.mod` (`github.com/rztaylor/flashheart`, Go 1.26.5+), `cmd/flashheart`,
  and the `internal/` packages listed in `.agents/facts/architecture.md`
  created only as they get real content (at least `cli`, `app`, `webui`,
  `config`). Each with `doc.go`.
- `internal/cli`: `serve` (default), `mcp`, `hook`, `setup`, `doctor`,
  `version` with `--help`; unimplemented commands exit 2 with "not yet
  available" (`CLI-1`, `CLI-3`). Global `--root` / `FLASHHEART_ROOT`
  resolution with `~` expansion and the default `~/reports/Kanban`.
- `version` prints app version, commit and `PROTOCOL_VERSION`.
- `internal/app` + `internal/webui`: Singleserve v0.2.x with
  `BrowserBoundLifetime`, browser open with manual URL fallback, embedded
  assets from `internal/webui/assets/generated/` (`LIFE-1`).
- `frontend/`: Vite + React + strict TypeScript + Tailwind v4 + Biome +
  Vitest + Playwright, per `.agents/facts/frontend-ui.md`. The shell connects
  with `/_singleserve/client.js`, shows quiet backend status, health check,
  Quit with denial handling, backend-lost and terminal states.
- `GET /api/info` (version, protocol version, root path) via `session.fetch`.
- `scripts/check.sh` made strict (no skipping), `scripts/build.sh`.
- `testdata/boards/` sample root (already drafted) loaded by a smoke test.
- `.github/` CI only once a remote exists (Decision needed: remote).

## Acceptance criteria

- `scripts/check.sh` passes from a clean clone: Biome, Vitest, frontend build,
  gofmt, `go vet`, `go test` (and `-race`), Go build.
- `flashheart --help`, `flashheart version`, unknown command → exit 2 with
  usage.
- `flashheart` opens the browser to the shell; reload, second tab, last-tab
  close, Quit, and backend loss behave per the Singleserve checklist
  (Playwright lifecycle test).
- No network access beyond loopback (test asserts the listener address).

## Out of scope

Any board reading or rendering; hooks; MCP.

## Validation

`scripts/check.sh`; Playwright lifecycle test at desktop width.
