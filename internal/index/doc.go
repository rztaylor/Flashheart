// Package index keeps serve's in-memory, revisioned snapshot of the board:
// every project with its derived blocking analysis and the agent runs of the
// last two days' event logs (read and folded incrementally across projects
// in time order, so a run with a claim in another project is one run, and
// refolded only when an event arrives out of order;
// linked to tickets by branch, and summarised per ticket: RUN-9) and each
// project's latest human board activity (EDIT-10; older activity is read
// once from the files before the window), rebuilt from store and events
// when a caller finds it older than MaxAge or when file watching sees a
// change.
//
// The revision increases only when the files read change (or the root
// appears or disappears), or when the clock changes a run's state. Watch
// turns external edits into new revisions within a second (STO-7) and Wait
// lets long-poll clients block until the revision moves (LIFE-3). Parsing and rules belong to board, run state to
// runs, file access to store and events, HTTP to api; hook and mcp modes
// never use index.
package index
