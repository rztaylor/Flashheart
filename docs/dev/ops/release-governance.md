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
- `setup` changes agent configuration without `--write`, or `--uninstall`
  leaves Flashheart entries behind or removes the user's own.
- A protocol change that is not additive ships without a `PROTOCOL_VERSION`
  bump and release-note instructions to re-run `setup`.
- Release notes overstate planned or unverified behaviour, or omit the agent
  versions the hook adapters were verified against.

## Candidate validation

Run `scripts/check.sh`, the Playwright suite in light and dark at 1280 and
1920, and the protocol smoke. Re-record golden hook payloads if either agent
has released since the last candidate.

## Artifacts

Manual until a hosted workflow is decided (`.agents/facts/release.md`). Never
overwrite an existing tag or artifact; issue a new version.
