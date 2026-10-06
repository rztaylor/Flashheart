// Package store is the only package that touches the board root. It reads
// v2 projects (PRJ-1) with their tickets, workstreams, reviews, files
// indexes, keys and archived ids, detects v1 roots and fingerprints what it
// read. It writes tickets and workstreams under a per-project lock with a
// content-hash precondition and atomic replace (STO-3), places tickets in
// their column's manual order (EDIT-9), keeps a ticket's
// workstream field and the workstreams' tickets lists in step and creates
// workstreams, creates tickets with the next id (KEY-2) and keys under a root lock (KEY-5), archives tickets,
// lists them and deletes archived ones permanently with their references
// (EDIT-8), copies allow-listed local files into a ticket (REV-1, REV-5),
// writes reviews (REV-4), finds or creates a repository's project
// (PRJ-2–PRJ-5), resolves working directories through the cwd cache, and
// keeps event files and answers inboxes.
//
// Every access goes through an os.Root with validated names (SEC-2).
// Parsing and blocking belong to board, text edits to mdfile, the event
// and inbox formats to events, revisions to index, and v1 conversion to
// migrate.
package store
