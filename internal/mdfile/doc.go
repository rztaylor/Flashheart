// Package mdfile parses and edits markdown files with YAML frontmatter: the
// frontmatter mapping in key order, the body, the H1 title, level-2 sections
// and top-level task-list checkboxes.
//
// It is pure (no I/O). Parsing never fails: frontmatter errors are returned
// on the Document so callers can show the file as needing repair (STO-4).
// Edits (set a field, tick a checkbox, replace or append to a section) touch
// only the lines they own and keep comments, key order, unknown keys and line
// endings; an edit that changes nothing returns the input unchanged (STO-2).
// Ticket meaning belongs to board; reading and writing files to store.
package mdfile
