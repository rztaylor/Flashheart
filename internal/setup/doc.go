// Package setup configures an agent for Flashheart (SET-1–SET-3, D9): for
// Claude Code, the hooks of agent-protocol §5.2 in ~/.claude/settings.json,
// the flashheart MCP server (registered through the claude CLI, which owns
// ~/.claude.json), the protocol skill, and moving the kanban-tracker skill
// aside. It plans first and renders the plan as a diff; applying takes
// timestamped backups under ~/.claude/flashheart-backup/, and uninstalling
// restores them when nothing changed since, else removes only its own
// entries. Settings keep their key order, value bytes, indent and line
// endings (layout is otherwise normalised); a symlinked settings.json is
// edited through to its target; settings in a shape it does not
// understand, or changed since the plan, are refused. It owns only hooks
// whose program is a flashheart binary run with hook. RefreshSkill keeps an
// installed protocol skill current for the session-start hook: it rewrites
// only an existing skill with Flashheart's frontmatter, never creates one.
// Diagnose checks the same configuration for doctor (SET-4) without
// changing it.
//
// Flags and output streams belong to cli; the skill's text to protocol.
// It never touches the board root.
package setup
