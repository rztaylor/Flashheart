# Codex hook payloads

Recorded payloads for the Codex hook adapter (agent-protocol §5.3 and §13).
Each `<Event>/<case>.json` is one hook payload as Codex sends it on stdin;
`mcp/` holds what Codex sent the flashheart MCP server in the same kind of
session. The adapter's expected events (`<case>.events.json`, as in
`testdata/hooks/claude`) arrive with the adapter.
`internal/hooks/codex` checks that every payload parses, names its event
and carries the hook session into its MCP calls.

Every payload is scrubbed: the working directory is `/Users/example/src/demo`,
Codex's home is `/Users/example/.codex`, session, subagent and turn ids are
replaced with fixed synthetic ones (the same in hooks and MCP calls, as in
the recordings), and prompts, replies, command output and the spawned
agent's (encrypted) task are replaced with synthetic text.

## Provenance

Recorded with `scripts/record-codex-hooks.sh exec` and `tui` from Codex CLI
0.160.0 (the copy bundled with the ChatGPT desktop app), model
`gpt-6.1-sol`, 2026-10-12, in a scratch repository with a scratch
`CODEX_HOME`; and with `app` from the ChatGPT desktop app (Codex 0.160.0,
`originator` `Codex Desktop`), 2026-10-12, in a scratch repository whose
`.codex/config.toml` held the hooks and the MCP server. The `app-*` cases
and `mcp/tools-call-app.json` come from the app.

| Event | CLI, `codex exec` | CLI, interactive | ChatGPT app |
|---|---|---|---|
| `SessionStart` | recorded: `startup`, `compact` | fired: `startup` | recorded: `app-startup` |
| `UserPromptSubmit` | recorded: `prompt` | fired | recorded: `app-prompt` |
| `PreToolUse` | recorded: `bash`, `apply-patch`, `flashheart-tool`, `spawn-agent`, `subagent-bash` | fired | recorded: `app-apply-patch`; `Bash` fired |
| `PermissionRequest` | recorded: `flashheart-tool` (an MCP tool's approval) | recorded: `bash` (a shell command's prompt, with the model's `description`) | not reached (no prompt needed approval) |
| `PostToolUse` | recorded: `bash`, `bash-failed`, `apply-patch`, `spawn-agent`, `wait-agent`, `subagent-bash` | fired | recorded: `app-apply-patch`; `Bash` fired |
| `SubagentStart` / `SubagentStop` | recorded: `default` | fired | not checked |
| `PreCompact` / `PostCompact` | recorded: `auto` (a low `model_auto_compact_token_limit`) | recorded: `PreCompact/manual` (`/compact`); `/quit` interrupted it before `PostCompact` | recorded: `app-manual` (`/compact`), both |
| `Stop` | recorded: `stop` | fired at each turn's end, not for an interrupted turn | recorded: `app-stop` |
| `SessionEnd` | recorded: `other`, when `codex exec` finishes | recorded: `quit` (`/quit`, `reason` `other`) | recorded: `app-archive` (archiving the thread, `reason` `other`) |
| `Interrupt` | not fired (nothing interrupts `exec`) | recorded: `esc` (Esc during a turn), `permission-denied` (denying a permission prompt ends the turn) | not checked |

Neither `codex exec` nor the interactive CLI offers a planning tool
(`update_plan`): asked to plan with it, the model said it had none. A failed
command is an ordinary `PostToolUse` with its output; there is no failure
event. An interrupted turn sends `Interrupt` and no `Stop`; `/compact` runs
as a turn of its own with no `UserPromptSubmit`. The interactive CLI starts
the MCP server when it opens, before the folder-trust prompt and
`SessionStart`. `resume` and `clear` session starts are not recorded.

The ChatGPT app ran the project's hooks only once they were trusted, and
it never offered to trust them: untrusted, they were skipped without a
prompt or a log line, while the same project's MCP server was used.
Trusting them with `/hooks` in the CLI (same `~/.codex`) made the app run
them; trust is saved in `~/.codex/config.toml` as `[hooks.state."<config
file>:<event>:<group>:<handler>"] trusted_hash = "sha256:…"`. The app's
payloads have the same fields as the CLI's. Archiving a thread sends
`SessionEnd` and moves its transcript to `~/.codex/archived_sessions/`, so
`transcript_path` stops existing. The app also starts an MCP server for
its own helper thread (the one that names a new thread), which never calls
a tool. Asked to plan, the model asked what to
plan instead, so whether the app offers `update_plan` is still open.

## Answers

**Where hooks are configured.** `hooks.json` or `[hooks]` in `config.toml`
in each config layer: the user's `$CODEX_HOME` (default `~/.codex`), a
project's `<repo>/.codex/` once the project is trusted, enabled plugins, and
managed `requirements.toml`. All layers' hooks run. Hooks are on by default
(`[features] hooks = false` turns them off). A hook that is not managed runs
only after the user trusts it (`/hooks` in the CLI); trust is tied to the
hook's hash, so a changed command must be trusted again.
`--dangerously-bypass-hook-trust` skips that for one invocation, which the
recordings use. `SessionEnd` and `Interrupt` timeouts are capped at 3 s.
Codex writes project trust to `$CODEX_HOME/config.toml` even with
`--ignore-user-config` or a `-c` override, which is why the recorder uses a
scratch `CODEX_HOME`.

**How hook output reaches the model** (checked 2026-10-12 with marker
text): `hookSpecificOutput.additionalContext` from `SessionStart` and
`UserPromptSubmit` is added as a developer message. A `Stop` hook printing
`{"decision":"block","reason":…}` continues the turn: the reason arrives as
a user message wrapped in `<hook_prompt hook_run_id="…">`, no
`UserPromptSubmit` fires for it, and the next `Stop` carries
`stop_hook_active: true`.

**Can `PreToolUse` rewrite a tool's input?** Yes. Printing
`hookSpecificOutput` with `permissionDecision: "allow"` and `updatedInput`
replaced a flashheart call's arguments: the server received the stamped
`run` (`mcp/tools-call-flashheart.json`), and the `PermissionRequest` that
followed already showed the rewritten input. `codex exec` refuses an MCP
call that needs approval; a `PermissionRequest` hook answering
`{"decision":{"behavior":"allow"}}` let it through.

**Do hooks and the MCP server share a session identifier?** Yes. Every
`tools/call` carries `_meta.sessionId` and `_meta.threadId`, and
`_meta["x-codex-turn-metadata"]` with `session_id`, `thread_id` and
`turn_id`; they equal the hooks' `session_id` and `turn_id`. A subagent's
hooks carry the parent's `session_id` with their own `agent_id`, `turn_id`
and transcript. Neither hooks nor the MCP server get any `CODEX_*`
environment variable. Codex starts one MCP server per session, in the
session's working directory, before `SessionStart` fires.

## Payload notes

- Every event has `session_id`, `cwd`, `hook_event_name` and
  `transcript_path`. All but `SessionEnd` have `model`; all but
  `SessionStart` and `SessionEnd` have `turn_id`; all but the compaction
  events and `SessionEnd` have `permission_mode`.
- `cwd` and the paths in `apply_patch` headers are absolute and resolved
  (`/private/var/…`, not `/var/…`).
- Tool names: `Bash` (`tool_input.command`), `apply_patch` (the whole patch
  in `tool_input.command`, files in `*** Add File:` / `*** Update File:`
  headers), MCP tools as `mcp__<server>__<tool>`, and the subagent tools as
  `collaborationspawn_agent` and `collaborationwait_agent`.
- `tool_use_id` is `exec-…` for a tool called from Codex's code mode and
  `call_…` otherwise.

Re-record with `scripts/record-codex-hooks.sh` when Codex changes its hook
schema. Recorded payloads win over the documentation (agent-protocol §1).
