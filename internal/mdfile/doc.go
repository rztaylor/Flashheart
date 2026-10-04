// Package mdfile parses markdown files with YAML frontmatter: the frontmatter
// mapping in key order, the body, the H1 title, level-2 sections and
// top-level task-list checkboxes.
//
// It is pure (no I/O) and never fails a parse: frontmatter errors are
// returned on the Document so callers can show the file as needing repair
// (STO-4). Ticket meaning (fields, columns, blocking) belongs to board;
// reading files belongs to store. Round-trip editing arrives with writes.
package mdfile
