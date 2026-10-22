// Package setup configures coding agents for Flashheart (SET-1–SET-4, D9).
//
// plan.go is the agent-neutral engine: a Plan of file changes, moves and
// commands, rendered as a diff; applying takes timestamped backups,
// replaces files atomically, refuses a file that changed since the plan,
// and runs the commands through the agent's CLI (or prints them). Agent is
// what each agent implements: Install, Uninstall, Diagnose (doctor's
// checks, as health.Findings), FindCLI and optionally RefreshProtocol
// (ProtocolRefresher), which the session-start hook calls through cli.
//
// Both agents get the same skill folder (skill.go, D89) and a marked
// section pointing at it in their global instructions file (section.go,
// edited through a symlink); the refresh keeps both current.
//
// Claude (claude*.go) is Claude Code: its hooks in
// ~/.claude/settings.json, edited preserving layout and owning only
// flashheart hook entries; the MCP server, registered through the claude
// CLI; ~/.claude/skills and CLAUDE.md. Uninstalling restores backups when
// nothing changed since, else removes only its own entries (D55).
//
// Codex (codex*.go) is Codex in $CODEX_HOME: the MCP server table in
// config.toml, edited as text (toml.go, no TOML library); its hooks in
// hooks.json; AGENTS.md; and ~/.agents/skills, where Codex reads user
// skills. It runs no Codex command; its diagnosis reports hooks Codex will
// skip by computing Codex's trust key and hash (codex_trust.go).
//
// Which agents exist, flags and output streams belong to cli; the text of
// the skill and of the section to protocol. It never touches the board root.
package setup
