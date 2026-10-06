// Package board is the pure domain model of a board: projects, tickets,
// workstreams and columns parsed from file contents, the blocking rules with
// reasons (CARD-4, board-format §Blocking), derived workstream status,
// format warnings and needs-repair reasons (STO-4), board order with manual
// ranks and placement plans (EDIT-9), and the editing rules
// the UI and the MCP tools share (editable fields, review warnings, the
// blocked-start note; EDIT-2, EDIT-3, EDIT-6).
//
// It performs no I/O and knows nothing about HTTP or the index: store reads
// files and hands their bytes here, and index composes the results.
package board
