// Package store is the only package that touches the board root. Its read
// side discovers v2 projects (PRJ-1), reads ticket folders, workstreams,
// reviews, files indexes, project keys and archived ids, detects v1 roots,
// and fingerprints what it read. Its write side edits tickets and
// workstreams under a per-project advisory lock with a content-hash
// precondition and atomic replace (STO-3), creates tickets with the next id
// (KEY-2), chooses project keys under a root lock (KEY-5), and archives and
// unarchives ticket folders (EDIT-8); confined primitives (atomic write,
// move) also serve whole-board operations such as migration.
//
// Every access goes through an os.Root, and names from callers are validated
// before use, so nothing escapes the root (SEC-2); unreadable or oversized
// files become needs-repair tickets rather than errors. Parsing and blocking
// belong to board, text edits to mdfile, caching and revisions to index, and
// the v1-to-v2 conversion to migrate.
package store
