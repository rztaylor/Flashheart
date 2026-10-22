// Package questionhistory keeps in each ticket's history what became of
// the questions agents asked about it (FH-190, board-format §Activity):
// serve runs it after each new snapshot, and for every question about a
// live ticket among the snapshot's runs it records, once each, that it was
// asked (by the asking run, when asked), answered on the board (by the
// person, when), answered in the session's own chat (by the asking run,
// at the prompt that answered it) or left open when its session ended (by
// the asking run, when serve saw it), through store.RecordQuestion, which
// writes nothing the history already has. ask_human and the board's answer
// write the first two themselves; this fills in the outcomes no write
// makes and anything a failed write or an older binary left out, while the
// run is in the run window. A failure is logged once and tried again on a
// later pass.
//
// Question folding belongs to runs, snapshots to index, the history's
// format to board and its writes to store.
package questionhistory
