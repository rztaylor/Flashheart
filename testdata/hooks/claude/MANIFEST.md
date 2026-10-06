# Claude Code hook payloads

Golden payloads for `internal/hooks/claude` (agent-protocol §13). Each
`<Event>/<case>.json` is one hook payload as Claude Code sends it on stdin;
`<case>.events.json` lists the events Flashheart must append for it (run,
kind and data; `ts` and `project` are checked by the test harness), and an
optional `<case>.output.json` is the hook output it must print (otherwise
none).

Every payload is scrubbed: the working directory is `/Users/example/src/demo`
(the test replaces it with a temporary git repository on branch
`feature/demo`), home and temporary paths are under `/Users/example`, and
prompts, replies, commands, command output and file contents are replaced
with synthetic text. Secrets that appear in fixtures are fake and exist to prove they are
never stored.

## Provenance

Checked 2026-10-05 against Claude Code 2.1.288.

| Source | Cases |
|---|---|
| Recorded from Claude Code 2.1.288 (CLI, 2026-10-05 and 2026-10-06) | `SessionStart/startup`, `SessionStart/startup-model`, `UserPromptSubmit/prompt`, `PreToolUse/bash`, `PostToolUse/edit`, `PostToolUse/write`, `PostToolUse/read`, `PostToolUse/bash`, `PostToolUse/agent`, `PostToolUse/subagent-bash`, `PostToolUseFailure/read-missing`, `PermissionRequest/bash`, `PreCompact/manual`, `Stop/stop`, `SubagentStart/explore`, `SubagentStop/explore`, `SubagentStop/helper`, `SessionEnd/other`, `SessionEnd/prompt-input-exit` |
| Recorded, then given a fake secret to prove commands are never stored | `PostToolUse/bash-secret` |
| Documented schema, not yet recorded | `SessionStart/compact`, `SessionStart/resume`, `PreToolUse/flashheart-tool`, `PostToolUse/multiedit`, `PostToolUse/notebookedit`, `PostToolUse/edit-outside-repo`, `PostToolUse/subagent-edit`, `PostToolUse/todowrite`, `PostToolUse/taskcreate`, `PostToolUse/taskupdate`, `PostToolUse/taskupdate-deleted`, `PostToolUseFailure/edit-failed`, `PermissionDenied/bash`, `Notification/*`, `TaskCreated/created`, `TaskCompleted/completed`, `PostCompact/manual` |

Not seen in the recordings: `Notification` (no idle or permission
notification reached the CLI hooks, though the desktop app's live check saw
permission notifications), `PostCompact` (a manual `/compact` sent only
`PreCompact`), `PermissionDenied` (denying a prompt does not send it; it is
for auto-mode denials) and the task-list tools (the model never made a list
through them). Recorded payloads carry fields the documented ones lack
(`effort`, `duration_ms`, `scratchpad_dir`, `background_tasks`,
`session_crons`, `model`); the adapter ignores them.

Live check, 2026-10-05: a session in the Claude desktop app's Code tab
(which runs the same Claude Code 2.1.288 with `--setting-sources
user,project,local`) produced, through `flashheart hook claude`, the events
these fixtures expect for session start and end, prompts, `Read`, `Write`,
`Edit` and `Bash` tool use, permission requests (with `tool_name`, including
one from inside an Explore subagent carrying `agent_id`), permission
notifications, an Explore subagent start, subagent stops and turn ends. The
app, like the CLI recording, sends `SubagentStop` for internal helper agents
it never reported starting; the run fold ignores those. No session has yet
exercised `TodoWrite` or the task tools for real.

Re-record with `scripts/record-claude-hooks.sh` when Claude Code changes its
hook schema, and move cases from the second row to the first as recordings
replace them. Recorded payloads win over the documentation
(agent-protocol §1).
