# Repository setup facts

- Foundation: `AGENTS.md`, `.agents/facts/`, `README.md`, `CHANGELOG.md`,
  `CONTRIBUTING.md`, `SECURITY.md`, `LICENSE` (MIT), `docs/`, `scripts/`,
  `testdata/`.
- GitHub: `.github/workflows/ci.yml` (read-only token; `scripts/check.sh` on
  Linux and macOS, `scripts/e2e.sh` on Linux with screenshots kept as an
  artifact), `.github/dependabot.yml` (weekly `gomod`, `npm` in `/frontend`,
  `github-actions`; minor and patch grouped), and a pull request template.
  Actions are pinned to full commit SHAs with the version in a comment.
- Repository settings (branch ruleset on `main`, delete branch on merge,
  Dependabot alerts, private vulnerability reporting) are applied with `gh`
  only with the owner's approval; record what was applied here.
- Caches (`.cache/`), build output (`build/`), `frontend/node_modules/` and
  generated web assets are ignored.
