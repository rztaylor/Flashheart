# Release governance

Flashheart is pre-release with no compatibility promise before v1.0.0.
Versions follow Semantic Versioning; tags are `vMAJOR.MINOR.PATCH`. The agent
protocol is versioned separately (`PROTOCOL_VERSION`,
`docs/dev/specs/agent-protocol.md` §14).

## Release blockers

- `scripts/check.sh`, the Playwright suite or the protocol end-to-end smoke
  fails or was skipped without an accepted reason.
- A hook can block or break an agent session other than through the
  documented, opt-in handoff enforcement.
- Hooks or logs store prompts, commands, tool inputs or outputs, or unscrubbed
  secrets.
- Any write outside the board root, or any deletion other than event-log
  retention.
- `setup` changes agent configuration without showing the diff and getting
  a yes (the prompt or `--yes`), or `--uninstall` leaves Flashheart entries
  behind or removes the user's own.
- A protocol change that is not additive ships without a `PROTOCOL_VERSION`
  bump and release-note instructions to re-run `setup`.
- Release notes overstate planned or unverified behaviour, or omit the agent
  versions the hook adapters were verified against.

## Candidate validation

The release workflow runs all of CI before it builds anything:
`scripts/check.sh` on Linux and macOS (which includes the protocol smoke)
and the Playwright suite with the screenshot comparison. Before cutting a
release, re-record golden hook payloads if either agent has released since
the last one, and check the changelog section against the blockers above.

## Artifacts

`.github/workflows/release.yml` (D86) builds and publishes every release;
nothing is uploaded by hand. A release holds, for darwin/arm64,
darwin/amd64, linux/amd64 and linux/arm64,
`flashheart_<version>_<os>_<arch>.tar.gz` (the binary, `LICENSE` and
`README.md`), and one `SHA256SUMS`. The tag is `v<version>` on the commit
pushed to `release`; the notes are the changelog section. Never overwrite
or delete an existing tag, release or artifact; issue a new version.

`scripts/package.sh` builds the same archives locally into `build/dist`
(`FLASHHEART_VERSION=0.1.0 scripts/package.sh`, or name targets such as
`darwin/arm64`); it publishes nothing.

## Cutting a release

1. On a feature branch, rename the changelog's `## Unreleased` section to
   `## [X.Y.Z] - YYYY-MM-DD` (today's date), curate its entries as the
   release notes, and add a new, empty `## Unreleased` above it. Check
   that `go run ./scripts/release version` prints `X.Y.Z` and
   `go run ./scripts/release notes` prints the notes. Merge it to `main`
   through a pull request as usual and wait for `main`'s CI to pass.
2. Move `release` to that commit. The first time, creating the branch
   from `main` starts the release:
   `git fetch origin && git push origin origin/main:refs/heads/release`.
   After that, open a pull request from `main` into `release`
   (`gh pr create --base release --head main --title "Release X.Y.Z"`) and
   merge it with a merge commit. Never commit to `release` directly, and
   never merge anything into it but `main`.
3. Watch the Release workflow (`gh run watch`). It stops before building
   if the version has no section, a heading is malformed, or `vX.Y.Z` is
   already tagged or released; it stops before publishing if any CI job
   fails. Fix the cause on `main` and merge `main` into `release` again,
   or re-run the failed jobs when the cause was transient. `gh release
   create` uploads into a draft and publishes it, with its tag, only once
   every file is up, so a publish that fails part-way leaves at most a
   draft, never seen publicly: delete that draft
   (`gh release delete vX.Y.Z`) before re-running, since the workflow
   refuses a version that has any release.
4. Check the published release: four archives and `SHA256SUMS`, notes as
   written. Download one, check it with
   `shasum -a 256 -c --ignore-missing SHA256SUMS`, and run
   `flashheart version`.

A push to `release` that adds no new version fails at the first step and
publishes nothing. Protect the `release` branch on GitHub (changes through
pull requests only) so it only ever moves to a commit of `main`.
