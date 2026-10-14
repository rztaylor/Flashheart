// Package store is the only package that touches the board root. Every
// access goes through an os.Root with validated names (SEC-2), so a symlink
// out of the root is refused; internal/archtest keeps other packages out.
//
// It reads projects (PRJ-1) with their tickets, workstreams, reviews,
// files indexes, keys and archived ids, and fingerprints what it read. It
// writes under a per-project lock in one transaction of atomic replaces
// (tx.go), each write naming its Precondition (STO-3), and a store writing
// as someone (As) appends the ticket's history in the same transaction
// (D47). Its writes: typed ticket edits from the board and agents alike
// (EditTicket), placement in manual order (EDIT-9), workstreams with their
// members, titles, cycle check and slug renames (EditWorkstream), new
// tickets and keys (KEY-2, KEY-5), archive, restore and permanent delete
// of tickets and projects with their references (EDIT-8, PRJ-5),
// attachments and files/index.yaml (REV-1, REV-5, REV-7), reviews and
// their ticks (REV-3, REV-4), people's comments and change requests in the
// activity file, project.yaml, config.yaml (ReadConfig,
// UpdateConfig) and the root itself (EnsureRoot). It finds or creates a
// repository's project, resolves working directories through the cwd
// cache, keeps event files, session inboxes and hooks' activity state, and
// expires them under the lock; WroteLast tells its own writes from others'.
//
// Parsing, blocking and history wording belong to board, text edits to
// mdfile, the event and inbox formats and which entries expire to events
// and app, and revisions to index.
package store
