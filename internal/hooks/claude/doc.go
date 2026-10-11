// Package claude is the Claude Code hook adapter (HOOK-7, agent-protocol
// §5.2): the only package that knows Claude Code's hook payloads and output
// format. It maps each hook event to Flashheart events, reading tool inputs
// only for edited file paths and plan items (HOOK-2), reports each tool
// result (which hooks throttles) and the checkpoint tool's results, stamps
// the run into Flashheart's own MCP tool calls at PreToolUse
// (agent-protocol §7.1), and renders additional context, updated tool input
// and stop decisions.
//
// Resolving projects, scrubbing, storing and the recovery note belong to
// hooks; the golden payloads it is tested against live in
// testdata/hooks/claude.
package claude
