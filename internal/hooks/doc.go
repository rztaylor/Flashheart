// Package hooks handles one agent hook invocation without knowing any
// agent's payload schema or tool names (HOOK-1–HOOK-4). It reads the
// payload, has the agent's Adapter turn it into events, resolves the
// project, branch and worktree from the working directory, throttles tool
// results into activity records kept in the session's state file under the
// project lock (agent-protocol §3), and appends the events.
//
// At session start it refreshes the protocol text through the caller and
// renders the recovery note; with a prompt or that note it delivers the
// session's inbox (HOOK-5, D54, through events.Deliver). At a turn end or
// end it marks a file changed since the last checkpoint (asking gitchange
// under a deadline; agent-protocol §4), blocks a stop once for a checkpoint
// where the project enforces handoffs (HOOK-6), and asks a session that
// changed files holding no ticket once to claim one (HOOK-8, D50). It
// folds only the sessions it asks about (runs.Keep, runs.Latest) to stay
// within HOOK-1's budget.
//
// Every failure is logged to hook-errors.log and swallowed. In an archived
// project's repository a hook records nothing; outside git it writes
// nothing at all, not even the root.
//
// Payload schemas and output formats belong to the adapters (hooks/claude,
// hooks/codex); the root, settings and files to store and events; run
// derivation to runs; git commands to gitchange; note wording to protocol.
// Hooks never use index, api, app or webui.
package hooks
