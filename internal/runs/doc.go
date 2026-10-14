// Package runs derives agent runs from events (RUN-1–RUN-5, agent-protocol
// §4). It folds a project's events into runs: start, end, turns, tools,
// edits since checkpoint and the shell-command windows within which a
// worktree change is the run's, claims, the run's home project (never one
// it only wrote a ticket in), questions and their answers, subagents and a
// bounded timeline. From the fold and a clock it derives each run's state
// (Working, Needs you, Waiting, Quiet, Ended), why a run needs you (Reason:
// typed, first match wins, never words; D39), its link to a ticket, a
// ticket's live claim holder (lease, §6), undelivered answers, whether a
// question's session has ended, whether an MCP-started run is still
// MCP-only, and its flags. The human's board writes (run `human`) are no
// run.
//
// It owns the run window every reader folds (Lookback) and what a partial
// fold needs: the events that derive some sessions exactly as a full fold
// would (Keep), and the last session in a worktree (Latest), so hooks
// decode only the sessions they ask about.
//
// It is pure: it reads events' types but no files. Reading the log belongs
// to events, finding in-progress tickets by branch to index, and
// presentation to api and the frontend.
package runs
