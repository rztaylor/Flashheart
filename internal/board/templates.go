package board

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// NewTicket is the content of a ticket created from the template (EDIT-5).
type NewTicket struct {
	Title       string
	Type        string
	Priority    string
	Status      Column
	Workstream  string
	Description string
	Criteria    []string
	DependsOn   []string
	Tags        []string
	Session     string
	// PlanOrRepro fills the Test Plan section, or Reproduction for a bug.
	PlanOrRepro string
	// Context fills the Context section: implementation detail kept below
	// the description, which is written for a person (FH-113).
	Context string
	// Key is used only when the project has no key yet (KEY-5).
	Key string
}

// Normalize collapses the title's spaces, fills the defaults (feature,
// medium, backlog) and checks every field, saying what is wrong.
func (t *NewTicket) Normalize() error {
	t.Title = strings.Join(strings.Fields(t.Title), " ")
	if t.Title == "" {
		return errors.New("a ticket needs a title")
	}
	if t.Type == "" {
		t.Type = "feature"
	}
	if !slices.Contains(TicketTypes, t.Type) {
		return fmt.Errorf("type must be one of %s", strings.Join(TicketTypes, ", "))
	}
	if t.Priority == "" {
		t.Priority = "medium"
	}
	if !slices.Contains(Priorities, t.Priority) {
		return fmt.Errorf("priority must be one of %s", strings.Join(Priorities, ", "))
	}
	if t.Status == "" {
		t.Status = Backlog
	}
	if _, ok := ParseColumn(string(t.Status)); !ok {
		return fmt.Errorf("status %q is not a column", t.Status)
	}
	if t.Workstream != "" && !ValidSlug(t.Workstream) {
		return fmt.Errorf("workstream %q", t.Workstream)
	}
	for _, id := range t.DependsOn {
		if _, _, ok := ParseID(id); !ok {
			return fmt.Errorf("depends-on %q is not a ticket id", id)
		}
	}
	return nil
}

// RenderTicket is a new ticket's file (board-format §Ticket): the
// frontmatter in its fixed order, then the sections of the template, with
// the description, plan and context kept inside theirs (mdfile.SectionSafe). t is
// normalized.
func RenderTicket(id string, t NewTicket, now time.Time) ([]byte, error) {
	body := []string{"# " + t.Title, "", "## " + SectionDescription, ""}
	if description := mdfile.SectionSafe(strings.TrimSpace(t.Description)); description != "" {
		body = append(body, description, "")
	}
	body = append(body, "## "+SectionCriteria, "")
	criteria := 0
	for _, criterion := range t.Criteria {
		if criterion = strings.Join(strings.Fields(criterion), " "); criterion != "" {
			body = append(body, "- [ ] "+criterion)
			criteria++
		}
	}
	if criteria > 0 {
		body = append(body, "")
	}
	if t.Type == "bug" {
		body = append(body, "## "+SectionReproduction, "")
	} else {
		body = append(body, "## "+SectionTestPlan, "")
	}
	if plan := mdfile.SectionSafe(strings.TrimSpace(t.PlanOrRepro)); plan != "" {
		body = append(body, plan, "")
	}
	body = append(body, "## "+SectionContext, "")
	if context := mdfile.SectionSafe(strings.TrimSpace(t.Context)); context != "" {
		body = append(body, context, "")
	}
	body = append(body, "## "+SectionNotes, "", "None.", "")
	return setFields([]byte("---\n---\n"+strings.Join(body, "\n")), []field{
		{key: "id", value: id},
		{key: "status", value: string(t.Status)},
		{key: "type", value: t.Type},
		{key: "priority", value: t.Priority},
		{key: "created", value: now.Format("2006-01-02")},
		{key: "branch"},
		{key: "workstream", value: t.Workstream},
		{key: "depends-on", list: t.DependsOn, isList: true},
		{key: "depends-on-workstreams", isList: true},
		{key: "tags", list: t.Tags, isList: true},
		{key: "session", value: t.Session},
		{key: "updated", value: now.Format(time.RFC3339)},
	})
}

// NewWorkstream is the content of a workstream created from the template.
type NewWorkstream struct {
	Title, Goal, Priority string
	// Tickets are live ticket ids of the project, in order; each joins the
	// new workstream and leaves any other.
	Tickets              []string
	DependsOnWorkstreams []string
	Tags                 []string
}

// Normalize collapses the title's spaces, fills the default priority and
// checks the fields.
func (w *NewWorkstream) Normalize() error {
	w.Title = strings.Join(strings.Fields(w.Title), " ")
	if w.Title == "" {
		return errors.New("a workstream needs a title")
	}
	if w.Priority == "" {
		w.Priority = "medium"
	}
	if !slices.Contains(Priorities, w.Priority) {
		return fmt.Errorf("priority must be one of %s", strings.Join(Priorities, ", "))
	}
	for index, id := range w.Tickets {
		if _, _, ok := ParseID(id); !ok {
			return fmt.Errorf("%q is not a ticket id", id)
		}
		if slices.Contains(w.Tickets[:index], id) {
			return fmt.Errorf("%s is listed twice", id)
		}
	}
	return nil
}

// RenderWorkstream is a new workstream's file (board-format §Workstream).
func RenderWorkstream(slug string, w NewWorkstream, now time.Time) ([]byte, error) {
	body := []string{"", "# " + w.Title, "", "## " + SectionGoal, ""}
	if goal := mdfile.SectionSafe(strings.TrimSpace(w.Goal)); goal != "" {
		body = append(body, goal, "")
	}
	body = append(body, "## "+SectionScope, "", "## "+SectionSuccessCriteria, "", "- [ ] Every ticket in the line reviewed", "", "## "+SectionNotes, "", "None.", "")
	// The skeleton fixes the field order and writes tickets as a block list,
	// as board-format shows it; SetList keeps that style.
	data := []byte("---\nslug: x\nstatus: x\npriority: x\ncreated: x\ntickets:\n  - x\ndepends-on-workstreams: []\ntags: []\n---\n" + strings.Join(body, "\n"))
	return setFields(data, []field{
		{key: "slug", value: slug},
		{key: "status", value: StatusActive},
		{key: "priority", value: w.Priority},
		{key: "created", value: now.Format("2006-01-02")},
		{key: "tickets", list: w.Tickets, isList: true},
		{key: "depends-on-workstreams", list: w.DependsOnWorkstreams, isList: true},
		{key: "tags", list: w.Tags, isList: true},
	})
}

type field struct {
	key    string
	value  string
	list   []string
	isList bool
}

func setFields(data []byte, fields []field) ([]byte, error) {
	var err error
	for _, f := range fields {
		if f.isList {
			data, err = mdfile.SetList(data, f.key, f.list)
		} else {
			data, err = mdfile.SetScalar(data, f.key, f.value)
		}
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}
