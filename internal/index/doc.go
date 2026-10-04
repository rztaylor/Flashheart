// Package index keeps serve's in-memory, revisioned snapshot of the board:
// every project with its derived blocking analysis, rebuilt from store when
// a caller finds it older than MaxAge.
//
// The revision increases only when the files read change (or the root
// appears or disappears), which later drives long-polling (LIFE-3). Parsing
// and rules belong to board, file access to store, HTTP to api. File
// watching arrives with board-editing; hook and mcp modes never use index.
package index
