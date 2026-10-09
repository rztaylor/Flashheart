// Package runs derives agent runs from events (RUN-1–RUN-5, agent-protocol
// §4): it folds a project's events into runs (start, end, turns, plan,
// tools, edits since checkpoint, which a session counts with its
// subagents' and where a turn end or end that found the worktree changed
// counts as one, the time windows of shell commands since checkpoint, within
// which such a change is the run's, claims and the claimant's home project, questions and their
// answers, on the board or in the session, subagents, a bounded timeline) and derives each run's state (Working,
// Needs you, Waiting, Quiet, Ended), its link to a ticket, a ticket's live
// claim holder (lease, agent-protocol §6), undelivered answers and its flags
// from the fold and a clock.
//
// It is pure: it reads events' types but no files. Reading the log belongs
// to events, finding in-progress tickets by branch to the caller (index),
// and presentation to api and the frontend.
package runs
