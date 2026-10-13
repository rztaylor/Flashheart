package board

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// corpus is the real FH tickets in testdata/tickets, each also with CRLF
// line endings, by name.
func corpus(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "tickets", "FH-*.md"))
	if err != nil || len(paths) < 5 {
		t.Fatalf("corpus: %d files, %v", len(paths), err)
	}
	files := map[string]string{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		files[filepath.Base(path)] = string(data)
		files[filepath.Base(path)+" (CRLF)"] = strings.ReplaceAll(string(data), "\n", "\r\n")
	}
	return files
}

// TestNewSectionChange: which sections a person may rewrite in place and
// how a section's text is checked (FH-66).
func TestNewSectionChange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, section, content string
		wantName, wantField    string
		readOnly               bool
		wantErr                string
	}{
		{name: "template section", section: "description", content: "Text.", wantName: SectionDescription, wantField: "description"},
		{name: "criteria", section: "Acceptance Criteria", wantName: SectionCriteria, wantField: "criteria"},
		{name: "test plan", section: " Test Plan ", wantName: SectionTestPlan, wantField: "test-plan"},
		{name: "reproduction", section: "Reproduction", wantName: SectionReproduction, wantField: "reproduction"},
		{name: "context", section: "CONTEXT", wantName: SectionContext, wantField: "context"},
		// A section of the ticket's own keeps its name; its words never
		// reach the event log.
		{name: "own section", section: "Design notes", wantName: "Design notes", wantField: "section"},
		{name: "handoff is the agents'", section: "Handoff", readOnly: true},
		{name: "notes are appended", section: "notes", readOnly: true},
		{name: "no name", section: "  ", wantErr: "a section needs a name"},
		{name: "two lines", section: "A\nB", wantErr: "one line"},
		{name: "a heading mark", section: "## Context", wantErr: "one line"},
		{name: "too long", section: "Context", content: strings.Repeat("x", MaxSectionText+1), wantErr: "at most"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			change, err := NewSectionChange(tc.section, tc.content)
			switch {
			case tc.readOnly:
				if !errors.Is(err, ErrReadOnlySection) {
					t.Fatalf("err = %v, want ErrReadOnlySection", err)
				}
			case tc.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
			case err != nil:
				t.Fatal(err)
			case change.Name() != tc.wantName || change.Field() != tc.wantField:
				t.Errorf("change = %q %q, want %q %q", change.Name(), change.Field(), tc.wantName, tc.wantField)
			}
		})
	}
}

// Rewriting one section of a real ticket changes the text between its
// heading and the next, and every other byte stays as it was; saving a
// section with its own text writes nothing (FH-66).
func TestSectionChangeLeavesTheRestOfARealTicket(t *testing.T) {
	t.Parallel()

	checked := 0
	for name, data := range corpus(t) {
		eol := "\n"
		if strings.Contains(data, "\r\n") {
			eol = "\r\n"
		}
		for _, section := range mdfile.Sections(mdfile.Parse([]byte(data)).Body) {
			if _, err := NewSectionChange(section.Heading, ""); errors.Is(err, ErrReadOnlySection) {
				continue
			}
			own, err := NewSectionChange(section.Heading, strings.ReplaceAll(section.Content, "\r\n", "\n"))
			if err != nil {
				t.Fatal(err)
			}
			if got, err := own.Apply([]byte(data)); err != nil || string(got) != data {
				t.Errorf("%s %s: saving its own text changed the file (%v)", name, section.Heading, err)
			}

			change, err := NewSectionChange(section.Heading, "Rewritten in place.\n\n- [ ] A new criterion\n")
			if err != nil {
				t.Fatal(err)
			}
			got, err := change.Apply([]byte(data))
			if err != nil {
				t.Fatal(err)
			}
			heading := "## " + section.Heading + eol
			start := strings.Index(data, heading) + len(heading)
			end := strings.Index(data[start:], eol+"## ")
			if end < 0 {
				end = len(data)
			} else {
				end += start + len(eol)
			}
			before, after := data[:start], data[end:]
			if !strings.HasPrefix(string(got), before) || !strings.HasSuffix(string(got), after) {
				t.Errorf("%s %s: bytes outside the section changed:\n%s", name, section.Heading, got)
				continue
			}
			middle := strings.TrimSuffix(strings.TrimPrefix(string(got), before), after)
			want := eol + "Rewritten in place." + eol + eol + "- [ ] A new criterion" + eol
			if after != "" {
				want += eol
			}
			if middle != want {
				t.Errorf("%s %s: section = %q, want %q", name, section.Heading, middle, want)
			}
			checked++
		}
	}
	if checked < 40 {
		t.Errorf("only %d sections checked", checked)
	}
}

// A field edit on a real ticket changes only that field's line (and the
// title only its heading); saving a field with its own value writes
// nothing (FH-66, STO-2).
func TestFieldChangeLeavesTheRestOfARealTicket(t *testing.T) {
	t.Parallel()

	edits := []FieldChange{
		{Key: "title", Value: "A title edited in place"},
		{Key: "priority", Value: "low"},
		{Key: "type", Value: "spike"},
		{Key: "branch", Value: "feature/in-place"},
		{Key: "tags", List: []string{"ui", "in-place"}},
		{Key: "depends-on", List: []string{"FH-3", "FH-7"}},
	}
	for name, data := range corpus(t) {
		lines := strings.SplitAfter(data, "\n")
		for _, edit := range edits {
			changes, err := NewFieldChanges([]FieldChange{edit})
			if err != nil {
				t.Fatal(err)
			}
			got, err := changes.Apply([]byte(data))
			if err != nil {
				t.Fatal(err)
			}
			after := strings.SplitAfter(string(got), "\n")
			if len(after) != len(lines) {
				t.Errorf("%s %s: %d lines, want %d", name, edit.Key, len(after), len(lines))
				continue
			}
			differ := 0
			for index := range lines {
				if lines[index] != after[index] {
					differ++
				}
			}
			if differ != 1 {
				t.Errorf("%s %s: %d lines changed, want 1", name, edit.Key, differ)
			}

			doc := mdfile.Parse([]byte(data))
			own := FieldChange{Key: edit.Key}
			switch edit.Key {
			case "title":
				own.Value = mdfile.Title(doc.Body)
			case "tags", "depends-on":
				own.List = doc.List(edit.Key)
			default:
				own.Value, _ = doc.String(edit.Key)
			}
			same, err := NewFieldChanges([]FieldChange{own})
			if err != nil {
				t.Fatal(err)
			}
			if unchanged, err := same.Apply([]byte(data)); err != nil || string(unchanged) != data {
				t.Errorf("%s %s: saving its own value changed the file (%v)", name, edit.Key, err)
			}
		}
	}
}

// A missing template section is added in the template's place, above the
// sections that follow it, so Add context lands above Notes and Handoff.
func TestSectionChangeAddsAMissingSectionInTheTemplatesPlace(t *testing.T) {
	t.Parallel()

	data := "---\nid: FH-1\n---\n# T\n\n## Description\n\nText.\n\n## Acceptance Criteria\n\n- [ ] One\n\n## Notes\n\nNone.\n\n## Handoff\n\nx\n"
	for _, tc := range []struct{ section, want string }{
		{"Context", "- [ ] One\n\n## Context\n\nAdded.\n\n## Notes"},
		{"test plan", "- [ ] One\n\n## Test Plan\n\nAdded.\n\n## Notes"},
		{"Design notes", "- [ ] One\n\n## Design notes\n\nAdded.\n\n## Notes"},
	} {
		change, err := NewSectionChange(tc.section, "Added.")
		if err != nil {
			t.Fatal(err)
		}
		got, err := change.Apply([]byte(data))
		if err != nil || !strings.Contains(string(got), tc.want) {
			t.Errorf("add %s =\n%s, %v; want it to contain\n%s", tc.section, got, err, tc.want)
		}
	}
	description, _ := NewSectionChange("Description", "Before the criteria.")
	got, _ := description.Apply([]byte("# T\n\n## Acceptance Criteria\n\n- [ ] One\n"))
	if want := "# T\n\n## Description\n\nBefore the criteria.\n\n## Acceptance Criteria\n\n- [ ] One\n"; string(got) != want {
		t.Errorf("add description = %q, want %q", got, want)
	}
}
