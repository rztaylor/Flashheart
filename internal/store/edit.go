package store

import "github.com/rztaylor/flashheart/internal/board"

// EditTicket is the one typed ticket edit the board and agents share
// (EDIT-6, FH-85): it writes changes (board.FieldChanges.Apply), then edit
// (nil for none, such as ticking criteria or appending a note), in one
// write once pre holds. A workstream change also makes that workstream the
// one that lists the ticket, in the same write (SetTicketWorkstream), so a
// refused write changes no file. It returns the ticket's hash after the
// call.
func (s *Store) EditTicket(project, id string, pre Precondition, changes board.FieldChanges, edit Edit) (string, error) {
	apply := func(data []byte) ([]byte, error) {
		data, err := changes.Apply(data)
		if err != nil || edit == nil {
			return data, err
		}
		return edit(data)
	}
	if workstream, ok := changes.Value("workstream"); ok {
		return s.SetTicketWorkstream(project, id, pre, workstream, apply)
	}
	return s.UpdateTicket(project, id, pre, apply)
}
