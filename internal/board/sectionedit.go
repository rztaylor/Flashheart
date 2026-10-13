package board

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Section edits: rewriting one section of a ticket in place, the board's
// in-place editor (FH-66), the one rule for which sections may be
// rewritten whole and where a missing one goes. store.EditTicket writes a
// SectionChange as its edit.

// ErrReadOnlySection is a section that is never rewritten whole: Handoff,
// which agents' checkpoints write, and Notes, which are appended to.
var ErrReadOnlySection = errors.New("this section is not rewritten in place")

// MaxSectionName is the most characters a section's name has.
const MaxSectionName = 100

// templateSections are the ticket template's sections in file order, each
// with the name a ticket.updated event records for it.
var templateSections = []struct{ name, field string }{
	{SectionDescription, "description"},
	{SectionCriteria, "criteria"},
	{SectionTestPlan, "test-plan"},
	{SectionReproduction, "reproduction"},
	{SectionContext, "context"},
	{SectionNotes, "notes"},
	{SectionHandoff, "handoff"},
}

// SectionChange is a checked rewrite of one level-2 section of a ticket.
// Only NewSectionChange makes one.
type SectionChange struct {
	name, field, content string
	// before are the sections a missing one is added above.
	before []string
}

// NewSectionChange checks a section rewrite: a name of one line (a
// template section's in any case takes the template's spelling), not
// Handoff or Notes (ErrReadOnlySection), and content of at most
// MaxSectionText bytes. Empty content clears the section.
func NewSectionChange(name, content string) (SectionChange, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return SectionChange{}, errors.New("a section needs a name")
	case strings.ContainsAny(name, "\r\n") || strings.HasPrefix(name, "#") || utf8.RuneCountInString(name) > MaxSectionName:
		return SectionChange{}, fmt.Errorf("a section's name is one line of at most %d characters", MaxSectionName)
	case len(content) > MaxSectionText:
		return SectionChange{}, fmt.Errorf("a section holds at most %d KiB; link a longer document instead", MaxSectionText>>10)
	}
	change := SectionChange{name: name, field: "section", content: content}
	for index, section := range templateSections {
		if strings.EqualFold(section.name, name) {
			if name = section.name; name == SectionHandoff || name == SectionNotes {
				return SectionChange{}, fmt.Errorf("%s: %w", name, ErrReadOnlySection)
			}
			change.name, change.field = name, section.field
			for _, later := range templateSections[index+1:] {
				change.before = append(change.before, later.name)
			}
			return change, nil
		}
	}
	change.before = []string{SectionNotes, SectionHandoff}
	return change, nil
}

// Name is the section's name.
func (c SectionChange) Name() string { return c.name }

// Field is what a ticket.updated event records for the change: the
// template section's key (criteria, description, test-plan, reproduction,
// context), or "section" for a section of the ticket's own, whose name is
// ticket text (agent-protocol §3).
func (c SectionChange) Field() string { return c.field }

// Apply rewrites the section in a ticket's text, leaving every other byte
// as it was; text equal to the section's own changes nothing. A missing
// section is added in the template's place: above the template sections
// that follow it, a section of the ticket's own above Notes and Handoff.
func (c SectionChange) Apply(data []byte) ([]byte, error) {
	return mdfile.PutSection(data, c.name, c.content, c.before...)
}
