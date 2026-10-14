package board

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// TestNewWorkstreamChanges: a workstream edit sets its title, goal,
// priority, tags and depends-on-workstreams, each checked as a ticket's
// field is; anything else, the slug and status included, is not editable
// (FH-46).
func TestNewWorkstreamChanges(t *testing.T) {
	t.Parallel()

	changes, err := NewWorkstreamChanges([]FieldChange{
		{Key: "tags", List: []string{" ui ", ""}},
		{Key: "title", Value: "  Card panel "},
		{Key: "goal", Value: "Open tickets beside the board."},
		{Key: "priority", Value: "high"},
		{Key: "depends-on-workstreams", List: []string{"board-ui"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := changes.Keys(); !slices.Equal(got, []string{"depends-on-workstreams", "goal", "priority", "tags", "title"}) {
		t.Errorf("keys = %v", got)
	}
	if list, ok := changes.List("depends-on-workstreams"); !ok || !slices.Equal(list, []string{"board-ui"}) {
		t.Errorf("depends-on-workstreams = %v, %v", list, ok)
	}
	if list, ok := changes.List("tags"); !ok || !slices.Equal(list, []string{"ui"}) {
		t.Errorf("tags = %v, %v", list, ok)
	}
	if _, ok := changes.List("nothing"); ok {
		t.Error("an unset key reads as set")
	}

	for name, tc := range map[string]struct {
		changes []FieldChange
		is      error
		says    string
	}{
		"slug":             {[]FieldChange{{Key: "slug", Value: "renamed"}}, ErrNotEditable, "slug"},
		"status":           {[]FieldChange{{Key: "status", Value: "completed"}}, ErrNotEditable, "status"},
		"ordered":          {[]FieldChange{{Key: "ordered", Value: "true"}}, ErrNotEditable, "ordered"},
		"tickets":          {[]FieldChange{{Key: "tickets", List: []string{"FH-1"}}}, ErrNotEditable, "tickets"},
		"a ticket field":   {[]FieldChange{{Key: "branch", Value: "main"}}, ErrNotEditable, "branch"},
		"empty title":      {[]FieldChange{{Key: "title", Value: "  "}}, nil, "title"},
		"long title":       {[]FieldChange{{Key: "title", Value: strings.Repeat("x", MaxTitle+1)}}, nil, "title"},
		"priority":         {[]FieldChange{{Key: "priority", Value: "urgent"}}, nil, "priority"},
		"dependency slug":  {[]FieldChange{{Key: "depends-on-workstreams", List: []string{"Not A Slug"}}}, nil, "not a workstream slug"},
		"tag on two lines": {[]FieldChange{{Key: "tags", List: []string{"a\nb"}}}, nil, "one line"},
		"goal too long":    {[]FieldChange{{Key: "goal", Value: strings.Repeat("x", MaxSectionText+1)}}, nil, "KiB"},
		"set twice":        {[]FieldChange{{Key: "title", Value: "A"}, {Key: "title", Value: "B"}}, nil, "twice"},
	} {
		_, err := NewWorkstreamChanges(tc.changes)
		if err == nil || !strings.Contains(err.Error(), tc.says) || (tc.is != nil && !errors.Is(err, tc.is)) {
			t.Errorf("%s: error = %v, want one saying %q", name, err, tc.says)
		}
	}

	if none, err := NewWorkstreamChanges(nil); err != nil || none.Len() != 0 {
		t.Errorf("no changes = %v, %v", none, err)
	}
}

// TestWorkstreamChangesLeaveTheRestOfARealWorkstream: each field edit
// rewrites only its line (the title its H1, the goal its section), the
// slug stays as it is, and an edit to the current value writes nothing.
func TestWorkstreamChangesLeaveTheRestOfARealWorkstream(t *testing.T) {
	t.Parallel()

	for name, data := range workstreamCorpus(t) {
		eol := "\n"
		if strings.Contains(data, "\r\n") {
			eol = "\r\n"
		}
		current := ParseWorkstream("x", []byte(data))
		goal, _ := mdfile.FindSection(strings.ReplaceAll(current.Body, "\r\n", "\n"), SectionGoal)
		same, err := NewWorkstreamChanges([]FieldChange{
			{Key: "title", Value: current.Title}, {Key: "priority", Value: current.Priority},
			{Key: "tags", List: current.Tags}, {Key: "depends-on-workstreams", List: current.DependsOnWorkstreams},
			{Key: "goal", Value: goal.Content},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, err := same.Apply([]byte(data)); err != nil || string(got) != data {
			t.Errorf("%s: setting the current values changed the file (%v)", name, err)
		}

		for _, tc := range []struct {
			change    FieldChange
			old, line string
		}{
			{FieldChange{Key: "title", Value: "Renamed line"}, "# " + current.Title + eol, "# Renamed line" + eol},
			{FieldChange{Key: "priority", Value: "low"}, "priority: " + current.Priority + eol, "priority: low" + eol},
		} {
			changes, err := NewWorkstreamChanges([]FieldChange{tc.change})
			if err != nil {
				t.Fatal(err)
			}
			got, err := changes.Apply([]byte(data))
			if err != nil {
				t.Fatal(err)
			}
			if want := strings.Replace(data, tc.old, tc.line, 1); string(got) != want {
				t.Errorf("%s %s: got\n%s\nwant\n%s", name, tc.change.Key, got, want)
			}
		}

		changes, err := NewWorkstreamChanges([]FieldChange{{Key: "goal", Value: "A new goal."}, {Key: "depends-on-workstreams", List: []string{"agent-runs"}}})
		if err != nil {
			t.Fatal(err)
		}
		got, err := changes.Apply([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		edited := ParseWorkstream("x", got)
		if !slices.Equal(edited.DependsOnWorkstreams, []string{"agent-runs"}) || edited.Title != current.Title || !slices.Equal(edited.Tickets, current.Tickets) {
			t.Errorf("%s: edited = %+v", name, edited)
		}
		if section, _ := mdfile.FindSection(strings.ReplaceAll(edited.Body, "\r\n", "\n"), SectionGoal); strings.TrimSpace(section.Content) != "A new goal." {
			t.Errorf("%s: goal = %q", name, section.Content)
		}
		slug, _ := mdfile.Parse(got).String("slug")
		if want, _ := mdfile.Parse([]byte(data)).String("slug"); slug != want {
			t.Errorf("%s: slug %q became %q", name, want, slug)
		}
	}
}

// TestWorkstreamCycle: a workstream's new dependencies are refused when
// they lead back to it, and the error names the way round (FH-46).
func TestWorkstreamCycle(t *testing.T) {
	t.Parallel()

	graph := map[string][]string{
		"api":     {"storage"},
		"storage": {"schema"},
		"schema":  nil,
		"ui":      {"api", "schema"},
		"broken":  {"missing"},
	}
	dependsOn := func(slug string) []string { return graph[slug] }
	for _, tc := range []struct {
		slug      string
		dependsOn []string
		cycle     string
	}{
		{"schema", []string{"schema"}, "schema → schema"},
		{"schema", []string{"api"}, "schema → api → storage → schema"},
		{"storage", []string{"schema", "ui"}, "storage → ui → api → storage"},
		{"schema", []string{"broken"}, ""},
		{"ui", []string{"api", "storage", "schema"}, ""},
		{"new", []string{"ui", "api"}, ""},
		{"api", nil, ""},
	} {
		err := WorkstreamCycle(tc.slug, tc.dependsOn, dependsOn)
		var cycle *CycleError
		switch {
		case tc.cycle == "" && err != nil:
			t.Errorf("%s → %v: unexpected %v", tc.slug, tc.dependsOn, err)
		case tc.cycle != "" && (!errors.As(err, &cycle) || strings.Join(cycle.Path, " → ") != tc.cycle):
			t.Errorf("%s → %v: error = %v, want the cycle %s", tc.slug, tc.dependsOn, err, tc.cycle)
		case tc.cycle != "" && !strings.Contains(err.Error(), tc.cycle):
			t.Errorf("error %q does not name the cycle %s", err, tc.cycle)
		}
	}
}
