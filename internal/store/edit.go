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
	return s.EditTicketAddressing(project, id, pre, changes, edit, nil)
}

// EditTicketAddressing is EditTicket that also marks the change requests
// addressed names (comment ids) addressed, in the same write, so an
// agent's update_ticket call is one history entry (FH-126). Only an
// agent addresses a request, and only one still change requested
// (board.CommentTransition): another is ErrInvalidInput and an unknown id
// ErrNotFound, and either writes nothing.
func (s *Store) EditTicketAddressing(project, id string, pre Precondition, changes board.FieldChanges, edit Edit, addressed []string) (string, error) {
	apply := func(data []byte) ([]byte, error) {
		data, err := changes.Apply(data)
		if err != nil || edit == nil {
			return data, err
		}
		return edit(data)
	}
	also := s.addressing(addressed)
	if workstream, ok := changes.Value("workstream"); ok {
		return s.setTicketWorkstream(project, id, pre, workstream, apply, also)
	}
	return s.updateTicket(project, id, pre, apply, also)
}
