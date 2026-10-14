package board

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

const rewriteTicket = "---\nid: FH-1\n---\n# T\n\n## Description\n\nText that mentions ## Handoff inline.\n\n" +
	"## Acceptance Criteria\n\n- [ ] One\n\n## Design notes\n\nOld.\n\n```\n## Context\n```\n\n## Notes\n\nNone.\n\n## Handoff\n\nx\n"

// Which sections an agent may rewrite (FH-69, D71): Handoff and Notes
// never, the criteria only by operation, a template section even when
// missing, and otherwise only a section the ticket has, so a misspelt
// name is refused naming the sections there are.
func TestNewAgentSectionChange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, section string
		want          string
		err           error
		message       string
	}{
		{name: "template section", section: "description", want: SectionDescription},
		{name: "missing template section is added", section: "Context", want: SectionContext},
		{name: "own section, as spelled in the ticket", section: "design NOTES", want: "Design notes"},
		{name: "handoff", section: "Handoff", err: ErrReadOnlySection},
		{name: "notes", section: "notes", err: ErrReadOnlySection},
		{name: "criteria", section: "Acceptance Criteria", err: ErrCriteriaSection},
		{name: "misspelt", section: "Descripton", err: ErrUnknownSection,
			message: "Description, Test Plan, Reproduction, Context, Design notes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			change, err := NewAgentSectionChange([]byte(rewriteTicket), tc.section, "New text.")
			switch {
			case tc.err != nil:
				if !errors.Is(err, tc.err) || !strings.Contains(err.Error(), tc.message) {
					t.Fatalf("err = %v, want %v naming %q", err, tc.err, tc.message)
				}
			case err != nil:
				t.Fatal(err)
			case change.Name() != tc.want:
				t.Errorf("name = %q, want %q", change.Name(), tc.want)
			}
		})
	}
}

// Headings are matched only at the start of a line outside code fences:
// text that mentions "## Handoff" is not a heading (FH-4), and a fenced
// "## Context" does not make the section exist.
func TestAgentSectionChangeMatchesLineAnchoredHeadings(t *testing.T) {
	t.Parallel()

	description, err := NewAgentSectionChange([]byte(rewriteTicket), "Description", "Rewritten.")
	if err != nil {
		t.Fatal(err)
	}
	got, err := description.Apply([]byte(rewriteTicket))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(rewriteTicket, "Text that mentions ## Handoff inline.", "Rewritten.", 1)
	if string(got) != want {
		t.Errorf("rewrite =\n%s\nwant\n%s", got, want)
	}
	context, err := NewAgentSectionChange([]byte(rewriteTicket), "Context", "Added.")
	if err != nil {
		t.Fatal(err)
	}
	got, err = context.Apply([]byte(rewriteTicket))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "```\n## Context\n```\n\n## Context\n\nAdded.\n\n## Notes") {
		t.Errorf("a fenced heading was taken for the section:\n%s", got)
	}
}

const criteriaTicket = "---\nid: FH-1\n---\n# T\n\n## Description\n\nText.\n\n## Acceptance Criteria\n\n" +
	"- [x] One\n- [ ] Two\n  more of two\n- [x] Three\n\n## Notes\n\nNone.\n"

// Criteria are edited by operation (FH-69, D71): items are named by their
// number before the call, so earlier operations never renumber later
// ones, and an item reworded or moved keeps its tick.
func TestCriteriaEdit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ops  []CriterionOp
		want string
	}{
		{"reword keeps the tick", []CriterionOp{{Kind: CriterionReword, Item: 0, Text: "First"}},
			"- [x] First\n- [ ] Two\n  more of two\n- [x] Three\n"},
		{"reword drops the old continuation", []CriterionOp{{Kind: CriterionReword, Item: 1, Text: "Second"}},
			"- [x] One\n- [ ] Second\n- [x] Three\n"},
		{"remove", []CriterionOp{{Kind: CriterionRemove, Item: 1}},
			"- [x] One\n- [x] Three\n"},
		{"move keeps the tick", []CriterionOp{{Kind: CriterionMove, Item: 2, Position: 1}},
			"- [x] Three\n- [x] One\n- [ ] Two\n  more of two\n"},
		{"add at the end", []CriterionOp{{Kind: CriterionAdd, Text: "Four"}},
			"- [x] One\n- [ ] Two\n  more of two\n- [x] Three\n- [ ] Four\n"},
		{"add in place", []CriterionOp{{Kind: CriterionAdd, Text: "Zero", Position: 1}},
			"- [ ] Zero\n- [x] One\n- [ ] Two\n  more of two\n- [x] Three\n"},
		{"numbers are the ticket's before the call", []CriterionOp{
			{Kind: CriterionRemove, Item: 0},
			{Kind: CriterionReword, Item: 2, Text: "Third"},
			{Kind: CriterionAdd, Text: "New", Position: 1},
		}, "- [ ] New\n- [ ] Two\n  more of two\n- [x] Third\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			edit, err := NewCriteriaEdit(3, tc.ops)
			if err != nil {
				t.Fatal(err)
			}
			got, err := edit.Apply([]byte(criteriaTicket))
			if err != nil {
				t.Fatal(err)
			}
			want := strings.Replace(criteriaTicket, "- [x] One\n- [ ] Two\n  more of two\n- [x] Three\n", tc.want, 1)
			if string(got) != want {
				t.Errorf("got\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func TestCriteriaEditRefusesWhatCannotBeDone(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ops  []CriterionOp
		want string
	}{
		{"no such criterion", []CriterionOp{{Kind: CriterionReword, Item: 3, Text: "x"}}, "no criterion 4"},
		{"removed earlier", []CriterionOp{{Kind: CriterionRemove, Item: 1}, {Kind: CriterionMove, Item: 1, Position: 1}}, "criterion 2 is removed earlier"},
		{"move needs a place", []CriterionOp{{Kind: CriterionMove, Item: 0}}, "position 1 to 3"},
		{"add past the end", []CriterionOp{{Kind: CriterionAdd, Text: "x", Position: 5}}, "position 1 to 4"},
		{"empty text", []CriterionOp{{Kind: CriterionReword, Item: 0, Text: "  "}}, "needs its text"},
		{"unknown operation", []CriterionOp{{Kind: "tick", Item: 0}}, "add, reword, remove or move"},
		{"nothing", nil, "no criteria operations"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewCriteriaEdit(3, tc.ops); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

// Criteria added to a ticket without any replace a placeholder, and a
// ticket without the section gets it in the template's place.
func TestCriteriaEditAddsTheFirstCriteria(t *testing.T) {
	t.Parallel()

	add, err := NewCriteriaEdit(0, []CriterionOp{{Kind: CriterionAdd, Text: "First"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, data, want string }{
		{"placeholder", "# T\n\n## Acceptance Criteria\n\nNone.\n\n## Notes\n", "# T\n\n## Acceptance Criteria\n\n- [ ] First\n\n## Notes\n"},
		{"intro kept", "# T\n\n## Acceptance Criteria\n\nThese hold:\n\n## Notes\n", "# T\n\n## Acceptance Criteria\n\nThese hold:\n\n- [ ] First\n\n## Notes\n"},
		{"missing", "# T\n\n## Description\n\nText.\n\n## Notes\n", "# T\n\n## Description\n\nText.\n\n## Acceptance Criteria\n\n- [ ] First\n\n## Notes\n"},
	} {
		got, err := add.Apply([]byte(tc.data))
		if err != nil || string(got) != tc.want {
			t.Errorf("%s: got %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
}

// Editing the criteria of a real ticket changes only its criteria section,
// and every criterion kept has the tick it had (FH-69).
func TestCriteriaEditLeavesTheRestOfARealTicket(t *testing.T) {
	t.Parallel()

	checked := 0
	for name, data := range corpus(t) {
		eol := "\n"
		if strings.Contains(data, "\r\n") {
			eol = "\r\n"
		}
		section, ok := mdfile.FindSection(mdfile.Parse([]byte(data)).Body, SectionCriteria)
		boxes := mdfile.Checkboxes(section.Content)
		if !ok || len(boxes) < 2 {
			continue
		}
		last := len(boxes) - 1
		edit, err := NewCriteriaEdit(len(boxes), []CriterionOp{
			{Kind: CriterionReword, Item: 0, Text: "Reworded"},
			{Kind: CriterionMove, Item: last, Position: 1},
			{Kind: CriterionAdd, Text: "Added"},
		})
		if err != nil {
			t.Fatal(err)
		}
		got, err := edit.Apply([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		heading := "## " + SectionCriteria + eol
		start := strings.Index(data, heading) + len(heading)
		end := strings.Index(data[start:], eol+"## ") + start + len(eol)
		before, after := data[:start], data[end:]
		if !strings.HasPrefix(string(got), before) || !strings.HasSuffix(string(got), after) {
			t.Errorf("%s: bytes outside the criteria changed", name)
			continue
		}
		rewritten, _ := mdfile.FindSection(mdfile.Parse(got).Body, SectionCriteria)
		now := mdfile.Checkboxes(rewritten.Content)
		want := append([]mdfile.Checkbox{boxes[last], {Text: "Reworded", Checked: boxes[0].Checked}}, boxes[1:last]...)
		want = append(want, mdfile.Checkbox{Text: "Added"})
		if len(now) != len(want) {
			t.Errorf("%s: %d criteria, want %d", name, len(now), len(want))
			continue
		}
		for index := range want {
			if now[index] != want[index] {
				t.Errorf("%s: criterion %d = %+v, want %+v", name, index+1, now[index], want[index])
			}
		}
		checked++
	}
	if checked < 4 {
		t.Errorf("only %d tickets checked", checked)
	}
}

// What a rewrite replaced is worked out from the ticket before and after
// it: the sections whose text changed, and the earlier text of those that
// had some, never the notes or the handoff (FH-69, D71).
func TestRewrittenSections(t *testing.T) {
	t.Parallel()

	before := []byte(rewriteTicket)
	after := []byte(strings.NewReplacer(
		"Text that mentions ## Handoff inline.", "New description.",
		"- [ ] One", "- [ ] One\n- [ ] Two",
		"## Notes\n\nNone.", "## Context\n\nAdded.\n\n## Notes\n\n- a note",
		"## Handoff\n\nx", "## Handoff\n\ny",
	).Replace(rewriteTicket))
	changed, earlier := Rewritten(before, after)
	if strings.Join(changed, ",") != "Description,Acceptance Criteria,Context" {
		t.Errorf("changed = %v", changed)
	}
	if len(earlier) != 2 || earlier[0].Heading != "Description" || earlier[1].Heading != "Acceptance Criteria" ||
		strings.TrimSpace(earlier[0].Content) != "Text that mentions ## Handoff inline." {
		t.Errorf("earlier = %+v", earlier)
	}
	if changed, earlier := Rewritten(before, before); changed != nil || earlier != nil {
		t.Errorf("no change = %v %v", changed, earlier)
	}

	at := time.Date(2026, 10, 14, 20, 42, 34, 0, time.UTC)
	text := string(EarlierText("FH-1", "claude:2ff6eb1c-0000", at, earlier))
	for _, part := range []string{
		"# FH-1: text replaced by claude:2ff6eb1c-0000 at 2026-10-14T20:42:34Z\n",
		"\n## Description\n\nText that mentions ## Handoff inline.\n",
		"\n## Acceptance Criteria\n\n- [ ] One\n",
	} {
		if !strings.Contains(text, part) {
			t.Errorf("earlier text is missing %q:\n%s", part, text)
		}
	}
	// Each kept section reads back as the section it was.
	for _, section := range earlier {
		kept, ok := mdfile.FindSection(mdfile.Parse([]byte(text)).Body, section.Heading)
		if !ok || strings.TrimSpace(kept.Content) != strings.TrimSpace(section.Content) {
			t.Errorf("%s reads back as %q", section.Heading, kept.Content)
		}
	}
}
