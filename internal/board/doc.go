// Package board is the pure domain model of a board: projects, tickets,
// workstreams and columns parsed from file contents, the blocking rules with
// reasons (CARD-4, board-format §Blocking), derived workstream status,
// format warnings and needs-repair reasons (STO-4), board order with manual
// ranks and placement plans (EDIT-9), and the editing rules
// the UI and the MCP tools share, each defined once (FH-85): editable
// fields and the checked change-set that writes them (fields.go), the
// title and section text limits, the blocked-start gate and its note,
// dated notes (NoteLine), which sections are rewritten whole and where a
// missing one goes (SectionChange, FH-66), review warnings and the project
// key rules, the
// shorter one agents choose included (EDIT-2, EDIT-3, EDIT-6, KEY-1,
// KEY-5), a review's How to Verify
// steps (REV-3), and local links in markdown with the attachment
// allow-list, its message and the attachment kinds, for copying the files
// they name (REV-2, REV-5), and what a file from the browser is by its
// bytes (SniffAttachment, SEC-7). It owns the file schema: section
// names (sections.go; nothing else names a section), the workstream slug
// rule (ValidSlug), new tickets' and workstreams' defaults, checks and
// templates (templates.go, rendered from values, no clock of its own), and
// the Handoff section's structure (HandoffNote, which RenderHandoff and
// ParseHandoff round-trip), and a ticket's activity file (activity.go:
// comment and history entries, parsed and written byte for byte, unknown
// entries kept; history.go: the changes a write made, worked out from the
// ticket and review before and after, never their text; D47), with the
// change requests in review (requests.go, REV-6, FH-126): who may move a
// comment's state where, the history of a state change worked out from
// the file's comments before and after, the counts a card shows and the
// warning that names open requests on a move to review.
//
// It performs no I/O and knows nothing about HTTP or the index: store reads
// files and hands their bytes here, and index composes the results.
package board
