// Package setup configures an agent for Flashheart (SET-1–SET-3, D9): for
// Claude Code, the hooks of agent-protocol §5.2 in ~/.claude/settings.json,
// the flashheart MCP server (registered through the claude CLI, which owns
// ~/.claude.json), the protocol skill, and moving the kanban-tracker skill
// aside. It plans first and renders the plan as a diff; applying takes
// timestamped backups under ~/.claude/flashheart-backup/, and uninstalling
// restores them when nothing changed since, else removes only its own
// entries. It edits settings order- and format-preserving and owns only
// hooks whose command runs a flashheart binary's hook subcommand.
//
// Flags and output streams belong to cli; the skill's text to protocol.
// It never touches the board root.
package setup
