// Package store is the only package that touches the board root. Its read
// side discovers v2 projects (PRJ-1), reads tickets, workstreams, reviews,
// files indexes, keys and archived ids, detects v1 roots and fingerprints
// what it read. Its write side edits tickets and workstreams under a
// per-project lock with a content-hash precondition and atomic replace
// (STO-3), creates tickets with the next id (KEY-2), chooses keys under a
// root lock (KEY-5), archives ticket folders (EDIT-8), finds or creates a
// repository's project (PRJ-2–PRJ-5), and appends, lists and expires event
// files; confined primitives also serve migration.
//
// Every access goes through an os.Root with validated names (SEC-2).
// Parsing and blocking belong to board, text edits to mdfile, the event
// format to events, revisions to index, and v1 conversion to migrate.
package store
