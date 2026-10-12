// Package index keeps serve's in-memory, revisioned snapshot of the board:
// every project with its derived blocking analysis and the agent runs of the
// last two days' event logs (read and folded incrementally across projects
// in time order, so a run with a claim in another project is one run, and
// refolded only when an event arrives out of order;
// linked to tickets by branch, and summarised per ticket: RUN-9) and each
// project's latest human board activity (EDIT-10; older activity is read
// once from the files before the window). Each snapshot indexes its runs by
// ticket, id and parent once, so per-card lookups do not scan every run.
//
// The board and the runs are fingerprinted apart: Rebuild reads both,
// RefreshRuns folds new events over the board already read (FH-96). The
// revision increases only when either fingerprint changes (or the root
// appears or disappears), or when the clock changes a run's state. Watch
// turns external edits into new revisions within a second (STO-7): a board
// change reads the board, unless a read begun since it was seen (an API
// write's own rebuild) already has it; an event append only refolds the
// runs, unless it records a ticket write. While Watch runs it is the
// freshness source and Current serves the snapshot as it is; without it
// Current rebuilds a snapshot whose board is older than MaxAge. Wait lets
// long-poll clients block until the revision moves (LIFE-3). Parsing and
// rules belong to board, run state to runs, file access to store and
// events, HTTP to api; hook and mcp modes never use index.
package index
