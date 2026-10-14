package mcpserver

import (
	"errors"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/scrub"
)

// rewrite is update_ticket's sections and criteria, decoded and checked
// against the ticket as read (FH-69, D75). Which sections may be
// rewritten and how criteria change are board's (NewAgentSectionChange,
// NewCriteriaEdit); keeping the replaced text is store's
// (TicketEdit.Rewrite). Its methods are safe on nil, a call without one.
type rewrite struct {
	c        *call
	sections []board.SectionChange
	criteria *board.CriteriaEdit
	// kept and wrote are set once the write keeps text and notes it.
	kept  string
	wrote bool
}

// agentSectionOrder orders rewritten sections as the template does.
var agentSectionOrder = []string{board.SectionDescription, board.SectionTestPlan, board.SectionReproduction, board.SectionContext}

// rewrite decodes update_ticket's sections and criteria; nil when the call
// has neither.
func (c *call) rewrite(project, id string, data []byte, current board.Ticket, sections map[string]string, criteria []CriterionInput) (*rewrite, error) {
	if len(sections) == 0 && len(criteria) == 0 {
		return nil, nil
	}
	r := &rewrite{c: c}
	for name, text := range sections {
		// The name is checked before local files the text links are
		// copied into the ticket, so a refused section copies nothing.
		change, err := board.NewAgentSectionChange(data, name, "")
		if err == nil {
			change, err = board.NewAgentSectionChange(data, name, c.copier(project, id).markdown(scrub.Secrets(text)))
		}
		switch {
		case errors.Is(err, board.ErrReadOnlySection) && strings.EqualFold(strings.TrimSpace(name), board.SectionHandoff):
			return nil, fail("invalid_input", "record your handoff with checkpoint", "%v", err)
		case errors.Is(err, board.ErrReadOnlySection):
			return nil, fail("invalid_input", "add to the notes with append_notes; earlier notes stay as written", "%v", err)
		case errors.Is(err, board.ErrCriteriaSection):
			return nil, fail("invalid_input", "edit the criteria with criteria (add, reword, remove, move)", "%v", err)
		case errors.Is(err, board.ErrUnknownSection):
			return nil, fail("invalid_input", "name a section the ticket has, or put the text in Context", "%v", err)
		case err != nil:
			return nil, fail("invalid_input", "correct the section", "%v", err)
		}
		if slices.ContainsFunc(r.sections, func(other board.SectionChange) bool { return other.Name() == change.Name() }) {
			return nil, fail("invalid_input", "name each section once", "%s is named twice", change.Name())
		}
		r.sections = append(r.sections, change)
	}
	slices.SortFunc(r.sections, func(a, b board.SectionChange) int {
		rank := func(change board.SectionChange) int {
			if at := slices.Index(agentSectionOrder, change.Name()); at >= 0 {
				return at
			}
			return len(agentSectionOrder)
		}
		if order := rank(a) - rank(b); order != 0 {
			return order
		}
		return strings.Compare(a.Name(), b.Name())
	})
	if len(criteria) > 0 {
		ops := make([]board.CriterionOp, 0, len(criteria))
		for _, input := range criteria {
			op := board.CriterionOp{Kind: strings.ToLower(strings.TrimSpace(input.Op)), Text: scrub.Text(input.Text, maxItemText), Position: input.Position}
			if op.Kind != board.CriterionAdd {
				if strings.TrimSpace(input.Item) == "" {
					return nil, fail("invalid_input", "name the criterion in item, by number (1 is the first) or text", "%s needs an item", input.Op)
				}
				index, err := criterionIndex(current.Criteria, input.Item)
				if err != nil {
					return nil, err
				}
				op.Item = index
			}
			ops = append(ops, op)
		}
		edit, err := board.NewCriteriaEdit(len(current.Criteria), ops)
		if err != nil {
			return nil, fail("invalid_input", "number criteria as get_ticket shows them before this call; positions count the list as the operations before leave it", "%v", err)
		}
		r.criteria = &edit
	}
	return r, nil
}

// edit is the rewrite as store applies it: sections, then criteria.
func (r *rewrite) edit(data []byte) ([]byte, error) {
	var err error
	for _, change := range r.sections {
		if data, err = change.Apply(data); err != nil {
			return nil, err
		}
	}
	if r.criteria != nil {
		return r.criteria.Apply(data)
	}
	return data, nil
}

// note is the ## Notes line recording the rewrite, given the file that
// keeps the replaced text: "Rewrote Description and Context; criteria: 2
// reworded. The replaced text is in [files/…](files/…).", under the
// caller's run.
func (r *rewrite) note(kept string) string {
	r.kept, r.wrote = kept, true
	var parts []string
	if len(r.sections) > 0 {
		names := make([]string, 0, len(r.sections))
		for _, change := range r.sections {
			names = append(names, change.Name())
		}
		parts = append(parts, "Rewrote "+andList(names))
	}
	if r.criteria != nil {
		parts = append(parts, r.criteria.Summary())
	}
	text := strings.Join(parts, "; ")
	if kept != "" {
		text += ". The replaced text is in [" + kept + "](" + kept + ")."
	}
	return r.c.note(text)
}

func (r *rewrite) noted() bool { return r != nil && r.wrote }

func (r *rewrite) keptFile() string {
	if r == nil {
		return ""
	}
	return r.kept
}

func (r *rewrite) changedCriteria() bool { return r.noted() && r.criteria != nil }

// changedSections is the event fields of the rewritten sections, once
// the write changed the ticket's text.
func (r *rewrite) changedSections() []string {
	if !r.noted() {
		return nil
	}
	var fields []string
	for _, change := range r.sections {
		if !slices.Contains(fields, change.Field()) {
			fields = append(fields, change.Field())
		}
	}
	return fields
}

// andList joins names as a sentence does: "A", "A and B", "A, B and C".
func andList(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
