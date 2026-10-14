package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/scrub"
)

// EditTicket is the one typed ticket edit the board and agents share
// (EDIT-6, FH-85): it writes changes (board.FieldChanges.Apply), then edit
// (nil for none, such as ticking criteria or appending a note), in one
// write once pre holds. A workstream change also makes that workstream the
// one that lists the ticket, in the same write (SetTicketWorkstream), so a
// refused write changes no file. It returns the ticket's hash after the
// call.
func (s *Store) EditTicket(project, id string, pre Precondition, changes board.FieldChanges, edit Edit) (string, error) {
	return s.ApplyTicketEdit(project, id, pre, TicketEdit{Changes: changes, Edit: edit})
}

// EditTicketAddressing is EditTicket that also marks the change requests
// addressed names (comment ids) addressed, in the same write, so an
// agent's update_ticket call is one history entry (FH-126); see
// TicketEdit.Addressed.
func (s *Store) EditTicketAddressing(project, id string, pre Precondition, changes board.FieldChanges, edit Edit, addressed []string) (string, error) {
	return s.ApplyTicketEdit(project, id, pre, TicketEdit{Changes: changes, Edit: edit, Addressed: addressed})
}

// TicketEdit is everything one update_ticket call or board edit writes to
// a ticket, in this order, in one write.
type TicketEdit struct {
	// Changes are the typed field changes.
	Changes board.FieldChanges
	// Edit (nil for none) follows them: ticks, an appended note.
	Edit Edit
	// Addressed are change requests (comment ids) marked addressed in the
	// ticket's activity file (FH-126). Only an agent addresses a request,
	// and only one still change requested (board.CommentTransition):
	// another is ErrInvalidInput and an unknown id ErrNotFound, and either
	// writes nothing.
	Addressed []string
	// Rewrite (nil for none) is an agent's rewrite of the ticket's text
	// (FH-69): sections replaced, criteria edited. The text of each
	// section it replaced is kept, as it was, in a markdown file in the
	// ticket's files/ (board.EarlierText, listed in files/index.yaml),
	// and Note's line, given that file's path relative to the ticket
	// folder ("" when the rewrite only added text), is appended to
	// ## Notes. A rewrite that changes no section writes neither.
	Rewrite Edit
	Note    func(kept string) string
}

// ApplyTicketEdit writes edit to a ticket once pre holds, under the
// project lock, in one write: the kept text of a rewrite and its index
// entry first, then the workstreams' lists for a workstream change
// (SetTicketWorkstream), then the ticket, then its activity file. It
// returns the ticket's hash after the call.
func (s *Store) ApplyTicketEdit(project, id string, pre Precondition, edit TicketEdit) (string, error) {
	var keep *keptText
	apply := func(data []byte) ([]byte, error) {
		data, err := edit.Changes.Apply(data)
		if err == nil && edit.Edit != nil {
			data, err = edit.Edit(data)
		}
		if err != nil || edit.Rewrite == nil {
			return data, err
		}
		return s.rewrite(project, id, data, edit, &keep)
	}
	addressing := s.addressing(edit.Addressed)
	also := func(t *tx, ticket string) error {
		if addressing != nil {
			if err := addressing(t, ticket); err != nil {
				return err
			}
		}
		if keep == nil {
			return nil
		}
		return s.stageKept(t, *keep)
	}
	if workstream, ok := edit.Changes.Value("workstream"); ok {
		return s.setTicketWorkstream(project, id, pre, workstream, apply, also)
	}
	return s.updateTicket(project, id, pre, apply, also)
}

// keptText is a rewrite's replaced text on its way to the ticket's files.
type keptText struct {
	dir, name, caption string
	data               []byte
	at                 time.Time
}

// keptOriginal is what a ticket's kept text is named after, before
// storedName's time and suffix.
const keptOriginal = "earlier-text.md"

// rewrite applies edit.Rewrite to data and, when it changed a section,
// keeps what it replaced and appends edit.Note's line. It runs inside the
// write, under the project lock, so the kept file's name is free.
func (s *Store) rewrite(project, id string, data []byte, edit TicketEdit, keep **keptText) ([]byte, error) {
	rewritten, err := edit.Rewrite(data)
	if err != nil {
		return nil, err
	}
	changed, earlier := board.Rewritten(data, rewritten)
	if len(changed) == 0 {
		return data, nil
	}
	kept := ""
	if len(earlier) > 0 {
		ticket, err := s.TicketFile(project, id)
		if err != nil {
			return nil, err
		}
		now, by := s.now(), s.author
		if by == "" {
			by = "unknown"
		}
		dir := path.Join(path.Dir(ticket), "files")
		names := make([]string, 0, len(earlier))
		for _, section := range earlier {
			names = append(names, section.Heading)
		}
		name := storedName(now, id+"-"+keptOriginal, func(name string) bool { return s.Exists(path.Join(dir, name)) })
		*keep = &keptText{
			dir: dir, name: name, at: now,
			caption: "Text replaced by " + shortRun(by) + " (" + strings.Join(names, ", ") + ")",
			data:    []byte(scrub.Secrets(string(board.EarlierText(id, by, now, earlier)))),
		}
		kept = "files/" + name
	}
	if edit.Note == nil {
		return rewritten, nil
	}
	note := edit.Note(kept)
	if note == "" {
		return rewritten, nil
	}
	return mdfile.AppendToSection(rewritten, board.SectionNotes, note)
}

// shortRun is a run id as a caption names it: agent and the session's
// first eight characters, or a person's name as it is.
func shortRun(by string) string {
	agent, session, ok := strings.Cut(by, ":")
	if !ok || len(session) <= 8 {
		return by
	}
	return agent + ":" + session[:8]
}

// stageKept stages a rewrite's kept text and its files index entry ahead
// of the ticket, so a crash between them leaves a listed file, not a note
// naming a missing one. An index that is not a list of files refuses the
// whole write.
func (s *Store) stageKept(t *tx, keep keptText) error {
	indexName := path.Join(keep.dir, "index.yaml")
	index, err := s.readIndex(indexName)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(keep.data)
	err = index.add(attachmentEntry{
		File: keep.name, Caption: board.OneLine(keep.caption, 300), Kind: board.AttachmentKindFor("other", keep.name),
		Run: s.author, Added: keep.at.Format(time.RFC3339), SHA256: hex.EncodeToString(sum[:]),
	})
	if err != nil {
		return err
	}
	encoded, err := index.encode()
	if err != nil {
		return fmt.Errorf("files index: %w", err)
	}
	t.files = slices.Insert(t.files, 0, staged{name: path.Join(keep.dir, keep.name), data: keep.data}, staged{name: indexName, data: encoded})
	return nil
}
