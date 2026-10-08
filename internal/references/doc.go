// Package references keeps the files that tickets and reviews edited
// directly (in a text editor rather than through Flashheart) link to
// (REV-5): serve runs it after each new snapshot, and for every ticket or
// review changed since the last one, other than by this serve process's own
// writes (so a raw edit sent from the browser cannot pull a local file into
// the board), it copies the allow-listed local files
// it links to from outside the board root into the ticket's files/
// (store.CopyIntoTicket, REV-2) and points the links at the copies, under
// the content-hash precondition. A synced board's edits look like the
// user's own, so a file is copied only from a checkout of the project's
// repository, found on disk with gitinfo and matched with
// store.FindProject, outside .git and not ignored by git, which it asks
// with `git check-ignore` (SEC-6). A link that cannot be copied stays as it
// is and is logged once.
//
// The first snapshot is only recorded, so edits made while serve was not
// running are left alone. Link syntax belongs to board, file access to
// store, snapshots to index.
package references
