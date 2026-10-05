// Package index keeps serve's in-memory, revisioned snapshot of the board:
// every project with its derived blocking analysis and the agent runs of the
// last two days' event logs (folded incrementally, linked to tickets by
// branch), rebuilt from store and events when a caller finds it older than
// MaxAge or when file watching sees a change.
//
// The revision increases only when the files read change (or the root
// appears or disappears), or when the clock changes a run's state. Watch
// turns external edits into new revisions within a second (STO-7) and Wait
// lets long-poll clients block until the revision moves (LIFE-3). Parsing and rules belong to board, run state to
// runs, file access to store and events, HTTP to api; hook and mcp modes
// never use index.
package index
