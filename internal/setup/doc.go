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
// Which agents exist, flags and output streams belong to cli; the skill's
// text to protocol. It never touches the board root.
package setup
