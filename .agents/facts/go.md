# Go facts

- Module: `github.com/rztaylor/flashheart`. Decision needed: GitHub repository
  name and visibility (`rztaylor/Flashheart` assumed).
- Minimum toolchain: Go 1.26.5, matching the Singleserve v0.2 line.
- Entrypoint: `cmd/flashheart`. Implementation under `internal/`; no public
  `pkg/` API is intended.
- Every hand-written package has a concise `doc.go` ownership contract.
- No CGO. Standard library by default; approved libraries are listed in
  `docs/dev/decisions.md` (D10).
- Generated web assets live under `internal/webui/assets/generated/` and are
  built before the Go build.
- Validation commands: `.agents/facts/testing.md`.
