// Package claude is the Claude Code hook adapter (HOOK-7, agent-protocol
// §5.2): the only package that knows Claude Code's hook payloads and output
// format. It maps each hook event to Flashheart events, reading tool inputs
// only for edited file paths and plan items (HOOK-2) and a prompt only for
// its origin (the user, a task notice or another agent; agent-protocol §3),
// reports each tool result classified as a shell command or an edit of its
// paths (which
// hooks throttles; Claude Code's tool names stay here) and the checkpoint
// tool's results, stamps the run into Flashheart's own MCP tool calls at
// PreToolUse (agent-protocol §7.1), says that a prompt's and a tool
// result's context reach the model (Input.Notes, for the claim reminder,
// D50), and renders additional context, updated tool input and stop
// decisions.
//
// Resolving projects, scrubbing, storing and the recovery note belong to
// hooks; the golden payloads it is tested against live in
// testdata/hooks/claude.
package claude
