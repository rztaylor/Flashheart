// Package gitchange answers one question for the turn-end and session-end
// hooks: did a git worktree's work change after a moment, other than through
// the edit tools whose paths hooks already record (agent-protocol §4, FH-53)?
// It asks git for the changed and untracked files and, when the worktree's
// HEAD moved since then, for the files of the commits made since, and
// compares their modification times with the moment. Read-only commands
// therefore leave no trace, and only the answer leaves the package: never a
// path, a command line or file content (HOOK-2).
//
// It runs git without optional locks, so it never competes with the agent's
// own git commands, under the caller's deadline (HOOK-1). Finding the
// worktree belongs to gitinfo; deciding when to ask, and recording the
// answer, to hooks and runs.
package gitchange
