// Package board is the pure domain model of a board: projects, tickets,
// workstreams and columns parsed from file contents, the blocking rules with
// reasons (CARD-4, board-format §Blocking), derived workstream status,
// format warnings and needs-repair reasons (STO-4), board order with manual
// ranks and placement plans (EDIT-9), and the editing rules
// the UI and the MCP tools share (editable fields, review warnings, dated
// notes and the blocked-start note, the section text limit; EDIT-2,
// EDIT-3, EDIT-6, FH-81), a review's How to Verify
// steps (REV-3), and local links in markdown with the attachment
// allow-list, its message and the attachment kinds, for copying the files
// they name (REV-2, REV-5). It owns the file schema: section
// names (sections.go; nothing else names a section), the workstream slug
// rule (ValidSlug), new tickets' and workstreams' defaults, checks and
// templates (templates.go, rendered from values, no clock of its own), and
// the Handoff section's structure (HandoffNote, which RenderHandoff and
// ParseHandoff round-trip).
//
// It performs no I/O and knows nothing about HTTP or the index: store reads
// files and hands their bytes here, and index composes the results.
package board
