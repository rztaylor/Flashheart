// Package events owns the event log (STO-5, agent-protocol §3): the envelope
// and its kinds with their data shapes, encoding one event per JSONL line,
// appending to a project's daily file under the project lock, tolerant and
// incremental reading (malformed lines and unknown kinds are skipped),
// expiring files after the retention period, and the answers inbox's line
// format (answers queued for a session's next prompt, HOOK-5) with the
// question.delivered events that record taking them, and the human's board
// activity: the shape of a board write's event (run `human`, EDIT-10) and
// which events count as the human acting on the board.
//
// File access goes through store, which confines, locks and names the files.
// What events mean for a run belongs to runs; turning hook payloads into
// events belongs to hooks and its agent adapters.
package events
