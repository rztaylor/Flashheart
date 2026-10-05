# Repository setup facts

- Foundation: `AGENTS.md`, `.agents/facts/`, `README.md`, `CHANGELOG.md`,
  `CONTRIBUTING.md`, `SECURITY.md`, `LICENSE` (MIT), `docs/`, `scripts/`,
  `testdata/`.
- GitHub: `.github/workflows/ci.yml` (read-only token; `scripts/check.sh` on
  Linux and macOS, `scripts/e2e.sh` on Linux with screenshots kept as an
  artifact), `.github/dependabot.yml` (weekly `gomod`, `npm` in `/frontend`,
  `github-actions`; minor and patch grouped), and a pull request template.
  Actions are pinned to full commit SHAs with the version in a comment.
- Repository settings are changed with `gh` only with the owner's approval.
  Applied 2026-10-05: ruleset "Protect main" on the default branch (pull
  request required with 0 approvals while there is one maintainer, stale
  reviews dismissed, conversations resolved; required checks
  `check (ubuntu-latest)`, `check (macos-latest)` and `browser suite`, up to
  date with `main`; no deletion or force push; no bypass actors); delete
  branch on merge; Actions token read-only and unable to approve pull
  requests; Dependabot alerts and security updates; private vulnerability
  reporting.
- Caches (`.cache/`), build output (`build/`), `frontend/node_modules/` and
  generated web assets are ignored.
