# Claude Code hook payloads

Golden payloads for `internal/hooks/claude` (agent-protocol §13). Each
`<Event>/<case>.json` is one hook payload as Claude Code sends it on stdin;
`<case>.events.json` lists the events Flashheart must append for it (run,
kind and data; `ts` and `project` are checked by the test harness).

Every payload is scrubbed: the working directory is `/Users/example/src/demo`
(the test replaces it with a temporary git repository on branch
`feature/demo`), home paths are `/Users/example`, and prompt text is
replaced. Secrets that appear in fixtures are fake and exist to prove they are
never stored.

## Provenance

Checked 2026-10-05 against Claude Code 2.1.288.

| Source | Cases |
|---|---|
| Recorded from Claude Code 2.1.288 | `SessionStart/startup`, `UserPromptSubmit/prompt`, `SessionEnd/other` |
| Documented schema, not yet recorded | every other case |

Re-record with `scripts/record-claude-hooks.sh` when Claude Code changes its
hook schema, and move cases from the second row to the first as recordings
replace them. Recorded payloads win over the documentation
(agent-protocol §1).
