// Package codex is the Codex hook adapter (HOOK-7, agent-protocol §5.3):
// the only package that knows Codex's hook payloads and output format. It
// maps each hook event to Flashheart events (an Interrupt ends the turn as
// a Stop does, but is never blocked), reading tool inputs only for the
// files an apply_patch names and update_plan's items (HOOK-2) and never a
// prompt, reports each tool result classified as a shell command or an
// edit of its paths (which hooks throttles; Codex's tool names stay here)
// and the checkpoint tool's results, says that only a prompt's context is
// known to reach the model (Input.Notes, for the claim reminder, D50),
// and renders additional context and stop decisions. It does not stamp the run into Flashheart's own tool
// calls: the MCP server reads the session from each call's _meta (D45).
//
// Resolving projects, scrubbing, storing and the recovery note belong to
// hooks. The golden payloads it is tested against, recorded from real
// Codex sessions, live in testdata/hooks/codex with what Codex sends the
// MCP server with a call.
package codex
