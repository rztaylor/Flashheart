// Package setup configures coding agents for Flashheart (SET-1–SET-4, D9).
//
// plan.go is the agent-neutral engine: a Plan of file changes, moves and
// commands, rendered as a diff; applying takes timestamped backups in the
// agent's backup folder, replaces files atomically, and runs the commands
// through the agent's CLI (or prints them when there is none). It refuses
// to write a file that changed since the plan. Agent is what each agent
// implements: Install, Uninstall, Diagnose (doctor's checks, as
// health.Findings) and FindCLI, and optionally RefreshSkill.
//
// Claude (claude.go, claude_diagnose.go, skill.go) is Claude Code: the
// hooks of agent-protocol §5.2 in ~/.claude/settings.json, the flashheart
// MCP server (registered through the claude CLI, which owns
// ~/.claude.json), the protocol skill, and moving the kanban-tracker skill
// aside; backups go under ~/.claude/flashheart-backup/. Uninstalling
// restores them when nothing changed since, else removes only its own
// entries. Settings keep their key order, value bytes, indent and line
// endings (layout is otherwise normalised); a symlinked settings.json is
// edited through to its target; settings in a shape it does not
// understand are refused. It owns only hooks whose program is a flashheart
// binary run with hook. RefreshSkill keeps an installed protocol skill
// current for the session-start hook: it rewrites only an existing skill
// with Flashheart's frontmatter, never creates one.
//
// Codex (codex*.go) is Codex, in $CODEX_HOME (default ~/.codex): the
// [mcp_servers.flashheart] table in config.toml, edited as text (toml.go
// scans statements, no TOML library) because Codex writes the file itself;
// the hooks of agent-protocol §5.3 in hooks.json, with Claude Code's JSON
// handling; and the protocol text between markers in the global
// AGENTS.md. It runs no Codex command. Its diagnosis also reports hooks
// Codex will skip (turned off, or never trusted with /hooks).
//
// Which agents exist, flags and output streams belong to cli; the skill's
// and AGENTS.md's text to protocol. It never touches the board root.
package setup
