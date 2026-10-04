# Release facts

- Maturity: pre-release, no compatibility promise before v1.0.0. The agent
  protocol has its own `PROTOCOL_VERSION`.
- Licence: MIT; text in `LICENSE`.
- Versioning: Semantic Versioning; tags `vMAJOR.MINOR.PATCH`.
- Changelog: `CHANGELOG.md`; governance: `docs/dev/ops/release-governance.md`.
  Release notes are curated from the changelog.
- Intended artifacts: macOS (arm64, amd64) and Linux (amd64, arm64) archives
  named `flashheart_<version>_<os>_<arch>.tar.gz`, with one `SHA256SUMS`.
- Decision needed: hosted release workflow, signing/notarisation, Homebrew
  tap, Windows. Releases are manual until decided.
- Release validation: `scripts/check.sh` plus the Playwright suite and the
  protocol end-to-end smoke.
