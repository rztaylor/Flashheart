# Release facts

- Maturity: pre-release, no compatibility promise before v1.0.0. The agent
  protocol has its own `PROTOCOL_VERSION`.
- Licence: MIT; text in `LICENSE`.
- Versioning: Semantic Versioning; tags `vMAJOR.MINOR.PATCH`.
- Changelog: `CHANGELOG.md`; governance: `docs/dev/ops/release-governance.md`.
  Release notes are curated from the changelog.
- Artifacts: macOS (arm64, amd64) and Linux (amd64, arm64) archives named
  `flashheart_<version>_<os>_<arch>.tar.gz` (version without the `v`),
  each holding `flashheart`, `LICENSE` and `README.md` at the top level,
  with one `SHA256SUMS` (`sha256sum` format). Built with `CGO_ENABLED=0`
  from one Linux runner; the binary is stamped with the version, short
  commit and UTC build time (`flashheart version`).
- Hosted release workflow (D86): `.github/workflows/release.yml` runs on
  every push to the `release` branch and nowhere else. The version is the
  newest `## [X.Y.Z] - YYYY-MM-DD` section of `CHANGELOG.md`
  (`go run ./scripts/release version`); the run fails before building when
  there is none, a heading is malformed, or tag `vX.Y.Z` or its release
  exists. It runs all of CI (`ci.yml`, called) first, builds the archives
  with `scripts/package.sh`, and only its publish job (`contents: write`,
  no repository code) creates the tag on the pushed commit and the GitHub
  release, notes from the changelog section. Not marked pre-release, so
  `releases/latest` finds it. Cut a release by finalising the changelog on
  main, then merging main into `release`
  (`docs/dev/ops/release-governance.md`). Nothing is ever committed to
  `release` directly.
- Local packaging: `scripts/package.sh` builds the same archives into
  `build/dist`; `scripts/release`'s tests run it for the host platform.
- Decision needed: signing/notarisation, Homebrew tap, Windows. Unsigned
  macOS binaries are ad-hoc linker-signed; a browser download needs its
  quarantine attribute cleared.
- Release validation: `scripts/check.sh` plus the Playwright suite and the
  protocol end-to-end smoke.
