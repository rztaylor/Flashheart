// Package store is the only package that touches the board root. Its read
// side discovers projects (PRJ-1), reads tickets, workstreams, reviews,
// attachment indexes and archived slugs, and fingerprints what it read.
//
// Every access goes through an os.Root, and names from callers are validated
// before use, so nothing escapes the root (SEC-2); unreadable or oversized
// files become needs-repair tickets rather than errors. Parsing and blocking
// belong to board; caching and revisions to index. Locked atomic writes
// arrive with board-editing.
package store
