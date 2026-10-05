// Package store is the only package that touches the board root. Its read
// side discovers v2 projects (PRJ-1), reads ticket folders, workstreams,
// reviews, files indexes, project keys and archived ids, detects v1 roots,
// and fingerprints what it read; confined primitives (atomic write, move)
// serve whole-board operations such as migration.
//
// Every access goes through an os.Root, and names from callers are validated
// before use, so nothing escapes the root (SEC-2); unreadable or oversized
// files become needs-repair tickets rather than errors. Parsing and blocking
// belong to board; caching and revisions to index; the v1-to-v2 conversion to
// migrate. Locked ticket writes arrive with board-editing.
package store
