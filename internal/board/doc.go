// Package board is the pure domain model of a board: projects, tickets,
// workstreams and columns parsed from file contents, the blocking rules
// with reasons (CARD-4), derived workstream status, format warnings and
// repair reasons (STO-4), and board order with manual ranks (EDIT-9).
//
// It defines once the editing rules the UI and the MCP tools share (FH-85):
// editable fields and their checked change-sets for tickets and
// workstreams, title and section limits, the blocked-start gate, dated
// notes, which sections are rewritten whole (SectionChange), the
// workstream cycle and title rules, review warnings, key rules, a review's
// How to Verify steps, local links and the attachment allow-list, and what
// a browser file is by its bytes (SniffAttachment, SEC-7).
//
// It owns the file schema: section names (sections.go; nothing else names
// a section), the slug rule, new files' defaults and templates (no clock of
// its own), the Handoff section's structure, and a ticket's activity file
// (D47): comment and history entries parsed and written byte for byte,
// the history a write made worked out from the files before and after,
// and the comment rules (change requests, REV-6; delivery to a session,
// FH-127; deletion, FH-140).
//
// It performs no I/O and knows nothing about HTTP or the index: store reads
// files and hands their bytes here, and index composes the results.
package board
