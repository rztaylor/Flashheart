// Package claude is the Claude Code hook adapter (HOOK-7, agent-protocol
// §5.2): the only package that knows Claude Code's hook payloads and output
// format. It maps each hook event to Flashheart events, reading tool inputs
// only for edited file paths and plan items (HOOK-2), and renders additional
// context as hookSpecificOutput.
//
// Resolving projects, scrubbing, storing and the recovery note belong to
// hooks; the golden payloads it is tested against live in
// testdata/hooks/claude.
package claude
