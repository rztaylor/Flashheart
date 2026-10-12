// Package store is the only package that touches the board root. It reads
// v2 projects (PRJ-1) with their tickets, workstreams, reviews (and when
// each last changed), files indexes, keys and archived ids, detects v1 roots and fingerprints what it
// read. It writes tickets and workstreams under a per-project lock, each
// write naming its Precondition (Unchanged with the content hash read, or
// Merge), in one transaction (tx.go: read, require, edit, stamp, stage,
// commit, nothing written when a step fails) of atomic replaces (STO-3),
// places tickets in
// their column's manual order (EDIT-9), keeps a ticket's
// workstream field and the workstreams' tickets lists in step and creates
// workstreams, creates tickets with the next id (KEY-2) and keys under a root lock (KEY-5), from board's templates, archives tickets,
// lists them and deletes archived ones permanently with their references
// (EDIT-8), archives, restores and permanently deletes projects with their
// references and retired keys (PRJ-5) (both deletes share one reference
// scan, run without a lock for the preview and again under the root lock
// to check its token), stores allow-listed, bounded,
// scrubbed files in a ticket from their bytes (AddAttachment) or a local
// path (CopyIntoTicket), with one reader and writer of files/index.yaml
// that keeps what it does not write (REV-1, REV-5),
// writes reviews (REV-4) and ticks their How to Verify steps (REV-3),
// reads every project.yaml through one parser that reports problems as
// warnings and edits it only through mdfile's plain-YAML editor (FH-77),
// finds or creates a repository's project
// (PRJ-2–PRJ-5; outside git there is none, ErrNoProject), resolves working
// directories through the cwd cache (never caching one outside git), and
// keeps event files, answers inboxes and the hooks' per-session activity
// state (read, appended with and replaced under one project lock).
//
// It also creates the root folder itself (EnsureRoot), reads and validates
// config.yaml (ReadConfig, defaults when it or the root is missing) and
// rewrites it under the root lock (UpdateConfig), and tells its own writes
// of a ticket or review from others' (WroteLast, by project, id and Doc).
//
// Callers that need a few projects at scale read every project's identity
// with ReadHeads; read-only callers find a repository's project with
// FindProject and its working directory with Locate, which write nothing.
//
// Every access goes through an os.Root with validated names (SEC-2), so a
// symlink out of the root, config.yaml's included, is refused. No other
// package opens files under the root (internal/archtest enforces this; the
// diagnostic logs, fsnotify watches and doctor's probe are the accepted
// exceptions, architecture.md).
// Parsing and blocking belong to board, text edits to mdfile, the event
// and inbox formats to events, revisions to index, and v1 conversion to
// migrate.
package store
