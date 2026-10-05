// Package index keeps serve's in-memory, revisioned snapshot of the board:
// every project with its derived blocking analysis, rebuilt from store when
// a caller finds it older than MaxAge or when file watching sees a change.
//
// The revision increases only when the files read change (or the root
// appears or disappears). Watch turns external edits into new revisions
// within a second (STO-7) and Wait lets long-poll clients block until the
// revision moves (LIFE-3). Parsing and rules belong to board, file access to
// store, HTTP to api; hook and mcp modes never use index.
package index
