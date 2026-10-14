// Package events owns the event log (STO-5, agent-protocol §3): the
// envelope and its kinds, each with its one data type (kinds.go), bounding
// and scrubbing every string on append (SEC-3, HOOK-2; no writer has to
// remember), the run-id grammar (RunID; nothing else cuts run ids apart),
// one event per JSONL line appended to a project's daily file under the
// project lock (alone or with a session's hook activity state), tolerant,
// incremental and selective reading (ReadSelected judges a line by its
// envelope before decoding its data), and expiring old files (FH-82).
//
// It owns the session inbox (inbox.go, D54): typed entries (answer,
// comment, change request) and their line format, Queue, Withdraw, and
// Deliver, the one routine every delivery path uses, which returns what it
// took even when recording delivery fails, so nothing taken is lost. It
// also owns the shape of the human's board activity events (run `human`,
// EDIT-10, D47).
//
// File access goes through store, which confines, locks and names the
// files. What events mean for a run belongs to runs; turning hook payloads
// into events to hooks and its adapters; the activity state's format to
// hooks.
package events
