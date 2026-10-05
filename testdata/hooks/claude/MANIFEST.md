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

Live check, 2026-10-05: a session in the Claude desktop app's Code tab
(which runs the same Claude Code 2.1.288 with `--setting-sources
user,project,local`) produced, through `flashheart hook claude`, the events
these fixtures expect for session start and end, prompts, `Read`, `Write`,
`Edit` and `Bash` tool use, permission requests (with `tool_name`, including
one from inside an Explore subagent carrying `agent_id`), permission
notifications, an Explore subagent start, subagent stops and turn ends. That
confirms the fields the adapter reads, but stores no raw payloads, so those
cases stay in the second row until recorded. The app also sends
`SubagentStop` for internal helper agents it never reported starting; the
run fold ignores those. No session has yet exercised `TodoWrite` or the task
tools for real.

Re-record with `scripts/record-claude-hooks.sh` when Claude Code changes its
hook schema, and move cases from the second row to the first as recordings
replace them. Recorded payloads win over the documentation
(agent-protocol §1).
