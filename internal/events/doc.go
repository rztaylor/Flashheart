// Package events owns the event log (STO-5, agent-protocol §3): the envelope
// and its kinds, each with its one data type (kinds.go: a kind with the
// wrong data is refused), bounding and scrubbing every string of every
// data type on append (SEC-3, HOOK-2; no writer has to remember), the
// run-id grammar (RunID,
// agent-protocol §2: nothing else cuts run ids apart), encoding one event per JSONL line,
// appending to a project's daily file under the project lock (alone, or
// with a session's hook activity state, whose format belongs to hooks),
// tolerant and
// incremental reading (malformed lines and unknown kinds are skipped),
// selective reading that judges a line by its envelope header before
// decoding its data (ReadSelected, or ReadHeaders for several passes over
// lines held in memory),
// expiring files after the retention period (with the session inboxes last
// written on those days, FH-82), and the session inbox (inbox.go, D54):
// its typed entries (answer, comment, change request) and line format,
// old untyped lines read as answers, queueing an entry (Queue), which
// entries a delivery takes (Mailbox), the question.delivered and
// comment.delivered events that record taking them, and Deliver, the one
// routine every delivery path uses (it takes inboxes, adds the fold's
// undelivered answers, records delivery and returns what it took even
// when recording fails, so nothing taken is lost), and the human's board
// activity: the shape of a board write's event (run `human`, EDIT-10;
// comment.added and comment.edited, D47) and which events count as the
// human acting on the board.
//
// File access goes through store, which confines, locks and names the files.
// What events mean for a run belongs to runs; turning hook payloads into
// events belongs to hooks and its agent adapters.
package events
