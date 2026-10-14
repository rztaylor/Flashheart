// Package mdfile parses and edits markdown files with YAML frontmatter: the
// frontmatter mapping in key order, the body, the H1 title, level-2 sections
// and top-level task-list checkboxes, with the lines each task item owns
// (TaskItems). Plain YAML mapping files such as
// project.yaml are parsed (ParseYAML) and edited (SetYAMLScalar,
// SetYAMLList) by the same field rules.
//
// It is pure (no I/O). Parsing never fails: frontmatter errors are returned
// on the Document so callers can show the file as needing repair (STO-4).
// Edits (set a field, tick a checkbox, replace, add in place or append to
// a section) touch
// only the lines they own and keep comments, key order, unknown keys and line
// endings; an edit that changes nothing returns the input unchanged (STO-2).
// Content written into a section stays in it (SectionSafe): its level-1
// and -2 headings become level 3 and an open code fence is closed.
// Ticket meaning belongs to board; reading and writing files to store.
package mdfile
