// Package index keeps serve's in-memory, revisioned snapshot of the board:
// every project with its derived blocking analysis, the agent runs of the
// run window (folded incrementally across projects in time order, linked
// to tickets by branch and summarised per ticket, RUN-9), and each
// project's latest human board activity (EDIT-10). Each snapshot indexes
// its runs by ticket, id and parent once, so per-card lookups never scan
// every run (D38), and knows each ticket's live claimants (Claimants) and
// the first of them (Holder), the one rule for who receives a change
// request or a comment (FH-126, FH-127).
//
// The board and the runs are fingerprinted apart: Rebuild reads both,
// RefreshRuns folds new events over the board already read (FH-96), and
// the revision moves only when either changes or the clock changes a run's
// state. Watch turns external edits into new revisions within a second
// (STO-7) and is then the freshness source; without it Current rebuilds on
// MaxAge. Wait lets long-poll clients block until the revision moves
// (LIFE-3). To add its fsnotify watches Watch stats and lists the root's
// directories itself, an accepted exception to store owning the root; it
// opens no file.
//
// Parsing and rules belong to board, run state to runs, file access to
// store and events, HTTP to api; hook and mcp modes never use index.
package index
