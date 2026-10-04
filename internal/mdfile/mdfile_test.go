package mdfile

import (
	"reflect"
	"strings"
	"testing"
)

const ticket = `---
type: feature
# a comment
priority: high
branch:
depends-on: [feat--a, feat--b]
tags: ui
custom: kept
---

# Card panel

## Description

Text with a [link](../todo/x.md).

` + "```" + `md
## Not a heading
- [ ] not a criterion
` + "```" + `

## Acceptance Criteria

- [x] First criterion
- [ ] Second *criterion*
  - [ ] nested child

## Handoff

**Next**
- Review tab
`

func TestParseSplitsFrontmatterAndBody(t *testing.T) {
	t.Parallel()

	doc := Parse([]byte(ticket))
	if doc.FrontmatterError != nil {
		t.Fatalf("FrontmatterError = %v", doc.FrontmatterError)
	}
	if !doc.HasFrontmatter {
		t.Fatal("HasFrontmatter = false")
	}
	if !strings.HasPrefix(doc.Body, "\n# Card panel") {
		t.Errorf("Body starts %q", doc.Body[:20])
	}
	if doc.BodyLine != 10 {
		t.Errorf("BodyLine = %d, want 10", doc.BodyLine)
	}
	if !strings.Contains(doc.FrontmatterRaw, "# a comment") {
		t.Errorf("FrontmatterRaw = %q", doc.FrontmatterRaw)
	}
}

func TestFieldsKeepOrderAndNormaliseValues(t *testing.T) {
	t.Parallel()

	doc := Parse([]byte(ticket))
	var keys []string
	for _, field := range doc.Fields() {
		keys = append(keys, field.Key)
	}
	if want := []string{"type", "priority", "branch", "depends-on", "tags", "custom"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("keys = %q, want %q", keys, want)
	}
	if got, _ := doc.String("priority"); got != "high" {
		t.Errorf("priority = %q", got)
	}
	if got, ok := doc.String("branch"); got != "" || !ok {
		t.Errorf("branch = %q, %v; want empty and present", got, ok)
	}
	if _, ok := doc.String("missing"); ok {
		t.Error("missing key reported present")
	}
	if got := doc.List("depends-on"); !reflect.DeepEqual(got, []string{"feat--a", "feat--b"}) {
		t.Errorf("depends-on = %q", got)
	}
	if got := doc.List("tags"); !reflect.DeepEqual(got, []string{"ui"}) {
		t.Errorf("scalar tags = %q, want one-item list", got)
	}
	if got := doc.List("branch"); got != nil {
		t.Errorf("empty list = %q, want nil", got)
	}
}

func TestParseTolerantInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		hasFront  bool
		wantError string
		body      string
	}{
		{"no frontmatter", "# Title\n", false, "", "# Title\n"},
		{"crlf and bom", "\ufeff---\r\ntype: bug\r\n---\r\n# T\r\n", true, "", "# T\n"},
		{"empty frontmatter", "---\n---\nbody\n", true, "", "body\n"},
		{"unclosed", "---\ntype: bug\n# T\n", true, "frontmatter is not closed", ""},
		// yaml.v3 locates most errors precisely (unclosed flow collections are
		// reported at line 1); lines are shifted to file lines.
		{"yaml error", "---\ntype: docs\nc: : bad\n---\n# T\n", true, "line 3", "# T\n"},
		{"unclosed flow", "---\ntype: docs\npriority: [unclosed\n---\n# T\n", true, "frontmatter does not parse", "# T\n"},
		{"not a mapping", "---\n- a\n- b\n---\nbody\n", true, "frontmatter must be a mapping", "body\n"},
		{"dashes later are body", "# T\n---\nnot: frontmatter\n---\n", false, "", "# T\n---\nnot: frontmatter\n---\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			doc := Parse([]byte(test.input))
			if doc.HasFrontmatter != test.hasFront {
				t.Errorf("HasFrontmatter = %v", doc.HasFrontmatter)
			}
			if test.wantError == "" && doc.FrontmatterError != nil {
				t.Errorf("FrontmatterError = %v", doc.FrontmatterError)
			}
			if test.wantError != "" && (doc.FrontmatterError == nil || !strings.Contains(doc.FrontmatterError.Error(), test.wantError)) {
				t.Errorf("FrontmatterError = %v, want %q", doc.FrontmatterError, test.wantError)
			}
			if test.body != "" && doc.Body != test.body {
				t.Errorf("Body = %q, want %q", doc.Body, test.body)
			}
		})
	}
}

func TestBrokenFrontmatterHasNoFields(t *testing.T) {
	t.Parallel()

	doc := Parse([]byte("---\npriority: [unclosed\n---\n# T\n"))
	if doc.Fields() != nil {
		t.Errorf("Fields() = %v, want nil for unparseable frontmatter", doc.Fields())
	}
}

func TestTitleAndSectionsIgnoreCodeFences(t *testing.T) {
	t.Parallel()

	doc := Parse([]byte(ticket))
	if got := Title(doc.Body); got != "Card panel" {
		t.Errorf("Title = %q", got)
	}
	var headings []string
	for _, section := range Sections(doc.Body) {
		headings = append(headings, section.Heading)
	}
	if want := []string{"Description", "Acceptance Criteria", "Handoff"}; !reflect.DeepEqual(headings, want) {
		t.Errorf("headings = %q, want %q", headings, want)
	}
	section, ok := FindSection(doc.Body, "acceptance criteria")
	if !ok || !strings.Contains(section.Content, "First criterion") || strings.Contains(section.Content, "## Handoff") {
		t.Errorf("FindSection(acceptance criteria) = %+v, %v", section, ok)
	}
}

func TestTitleFallsBackToEmpty(t *testing.T) {
	t.Parallel()

	if got := Title("no heading here\n## Sub\n"); got != "" {
		t.Errorf("Title = %q", got)
	}
	if got := Title("#  Spaced   title  \n"); got != "Spaced   title" {
		t.Errorf("Title = %q", got)
	}
}

func TestCheckboxesAreTopLevelTaskItems(t *testing.T) {
	t.Parallel()

	section, _ := FindSection(Parse([]byte(ticket)).Body, "Acceptance Criteria")
	got := Checkboxes(section.Content)
	want := []Checkbox{{Text: "First criterion", Checked: true}, {Text: "Second *criterion*", Checked: false}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Checkboxes = %+v, want %+v", got, want)
	}
	if got := Checkboxes("* [X] star bullet\n1. [ ] numbered is not a task\n"); !reflect.DeepEqual(got, []Checkbox{{Text: "star bullet", Checked: true}}) {
		t.Errorf("Checkboxes = %+v", got)
	}
}
