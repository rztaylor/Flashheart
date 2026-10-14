// Package runs derives agent runs from events (RUN-1–RUN-5, agent-protocol
// §4): it folds a project's events into runs (start, end, turns,
// tools from activity records and, in older logs, tool.used, edits since
// checkpoint, which a session counts with its
// subagents' and where a turn end or end that found the worktree changed
// counts as one, the time windows of shell commands since checkpoint (as
// adapters classify them; Bash and PowerShell in older tool.used), within
// which such a change is the run's, claims and the claimant's home project, the run's home project (never
// where it only wrote to another project's ticket), questions and their
// answers, on the board or in the session, subagents, a bounded timeline) and derives each run's state (Working,
// Needs you, Waiting, Quiet, Ended) with why a run in Needs you needs you
// (Reason: typed, first match wins, never words), its link to a ticket, a ticket's live
// claim holder (lease, agent-protocol §6), undelivered answers, whether a
// question's session (a subagent's is its session's) has ended, whether a
// run the MCP server started is still MCP-only (no event only hooks
// record), and its flags from the fold and a clock. The human's board writes (run `human`) are no
// run and are skipped.
//
// It owns the run window every reader folds (Lookback) and what a partial
// fold needs: the events that derive some sessions exactly as a full fold
// would (Keep), and the last session in a worktree found from the events'
// headers (Latest), so hooks decode only the sessions they ask about.
//
// It is pure: it reads events' types but no files. Reading the log belongs
// to events, finding in-progress tickets by branch to the caller (index),
// and presentation to api and the frontend.
package runs
