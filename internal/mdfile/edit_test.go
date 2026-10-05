package mdfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestSetScalarReplacesOnlyThatField(t *testing.T) {
	t.Parallel()

	got, err := SetScalar([]byte(ticket), "priority", "low")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(ticket, "priority: high\n", "priority: low\n", 1)
	if string(got) != want {
		t.Errorf("SetScalar =\n%s\nwant\n%s", got, want)
	}
}

func TestSetScalarKeepsCommentsBetweenFields(t *testing.T) {
	t.Parallel()

	// The comment above priority belongs to the gap after type.
	got, err := SetScalar([]byte(ticket), "type", "bug")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "type: bug\n# a comment\npriority: high\n") {
		t.Errorf("SetScalar =\n%s", got)
	}
}

func TestSetScalarQuotesWhenPlainWouldChangeMeaning(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"feature/x":            "branch: feature/x\n",
		"2026-10-05":           "branch: 2026-10-05\n",
		"a: b":                 `branch: "a: b"` + "\n",
		"# hash":               `branch: "# hash"` + "\n",
		"null":                 `branch: "null"` + "\n",
		"":                     "branch:\n",
		"line\nbreak":          `branch: "line\nbreak"` + "\n",
		" padded":              `branch: " padded"` + "\n",
		"it's <fine> & \"ok\"": `branch: "it's <fine> & \"ok\""` + "\n",
	}
	for value, line := range tests {
		got, err := SetScalar([]byte(ticket), "branch", value)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), "\n"+line) {
			t.Errorf("SetScalar(%q) =\n%s\nwant line %q", value, got, line)
		}
		if read, _ := Parse(got).String("branch"); read != strings.TrimSpace(value) && read != value {
			t.Errorf("SetScalar(%q) reads back as %q", value, read)
		}
	}
}

func TestSetScalarAppendsMissingFieldAndReplacesBlockValues(t *testing.T) {
	t.Parallel()

	data := "---\ntype: bug\ntags:\n  - a\n  - b\n# trailing comment\n---\n# T\n"
	got, err := SetScalar([]byte(data), "status", "done")
	if err != nil {
		t.Fatal(err)
	}
	if want := "---\ntype: bug\ntags:\n  - a\n  - b\n# trailing comment\nstatus: done\n---\n# T\n"; string(got) != want {
		t.Errorf("append =\n%q\nwant\n%q", got, want)
	}
	got, err = SetList([]byte(data), "tags", []string{"x", "needs, comma"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "---\ntype: bug\ntags:\n  - x\n  - \"needs, comma\"\n# trailing comment\n---\n# T\n"; string(got) != want {
		t.Errorf("SetList =\n%q\nwant\n%q", got, want)
	}
}

func TestSetFieldCreatesFrontmatterAndRefusesBrokenOne(t *testing.T) {
	t.Parallel()

	got, err := SetScalar([]byte("# Title\n"), "status", "backlog")
	if err != nil || string(got) != "---\nstatus: backlog\n---\n# Title\n" {
		t.Errorf("SetScalar(no frontmatter) = %q, %v", got, err)
	}
	if _, err := SetScalar([]byte("---\ntags: [oops\n---\n# T\n"), "status", "done"); !errors.Is(err, ErrBrokenFrontmatter) {
		t.Errorf("SetScalar(broken) error = %v", err)
	}
}

func TestEditsKeepLineEndingsAndBOM(t *testing.T) {
	t.Parallel()

	data := "\ufeff---\r\nstatus: backlog\r\n---\r\n# T\r\n"
	got, err := SetScalar([]byte(data), "status", "done")
	if err != nil {
		t.Fatal(err)
	}
	if want := "\ufeff---\r\nstatus: done\r\n---\r\n# T\r\n"; string(got) != want {
		t.Errorf("SetScalar = %q, want %q", got, want)
	}
}

func TestSetCheckbox(t *testing.T) {
	t.Parallel()

	got, err := SetCheckbox([]byte(ticket), "Acceptance Criteria", 1, true)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(ticket, "- [ ] Second *criterion*", "- [x] Second *criterion*", 1)
	if string(got) != want {
		t.Errorf("SetCheckbox =\n%s", got)
	}
	got, err = SetCheckbox([]byte(ticket), "acceptance criteria", 0, false)
	if err != nil || !strings.Contains(string(got), "- [ ] First criterion") {
		t.Errorf("untick = %v\n%s", err, got)
	}
	// The fenced pseudo-criterion and the nested child never count.
	if _, err := SetCheckbox([]byte(ticket), "Acceptance Criteria", 2, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("index 2 error = %v", err)
	}
	if _, err := SetCheckbox([]byte(ticket), "Nope", 0, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing section error = %v", err)
	}
}

func TestReplaceSection(t *testing.T) {
	t.Parallel()

	got, err := ReplaceSection([]byte(ticket), "Handoff", "**Next**\n- Ship it\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(got), "## Handoff\n\n**Next**\n- Ship it\n") {
		t.Errorf("ReplaceSection(last) =\n%s", got)
	}
	got, err = ReplaceSection([]byte(ticket), "Description", "New text.")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "## Description\n\nNew text.\n\n## Acceptance Criteria\n") {
		t.Errorf("ReplaceSection(middle) =\n%s", got)
	}
	got, err = ReplaceSection([]byte("---\na: b\n---\n# T\n"), "Notes", "Added.\n")
	if err != nil || string(got) != "---\na: b\n---\n# T\n\n## Notes\n\nAdded.\n" {
		t.Errorf("ReplaceSection(new) = %q, %v", got, err)
	}
}

func TestAppendToSection(t *testing.T) {
	t.Parallel()

	data := "# T\n\n## Notes\n\nNone.\n\n## Handoff\n\nx\n"
	got, err := AppendToSection([]byte(data), "Notes", "Moved while blocked: urgent.")
	if err != nil {
		t.Fatal(err)
	}
	if want := "# T\n\n## Notes\n\nMoved while blocked: urgent.\n\n## Handoff\n\nx\n"; string(got) != want {
		t.Errorf("append over placeholder =\n%q\nwant\n%q", got, want)
	}
	got, err = AppendToSection(got, "Notes", "Second.")
	if err != nil {
		t.Fatal(err)
	}
	if want := "# T\n\n## Notes\n\nMoved while blocked: urgent.\n\nSecond.\n\n## Handoff\n\nx\n"; string(got) != want {
		t.Errorf("append =\n%q\nwant\n%q", got, want)
	}
}

// Saving a field with its current value never changes a file, so a save
// with nothing edited is byte-identical for every fixture (board-editing).
func TestNoOpEditsAreByteIdenticalForEveryFixture(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "testdata", "boards")
	count := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		doc := Parse(data)
		if doc.FrontmatterError != nil {
			return nil
		}
		for _, field := range doc.Fields() {
			var got []byte
			if field.node.Kind == yaml.SequenceNode {
				got, err = SetList(data, field.Key, doc.List(field.Key))
			} else {
				value, _ := doc.String(field.Key)
				got, err = SetScalar(data, field.Key, value)
			}
			if err != nil {
				t.Errorf("%s %s: %v", path, field.Key, err)
				continue
			}
			if string(got) != string(data) {
				t.Errorf("%s: saving %s unchanged rewrote the file", path, field.Key)
			}
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count < 50 {
		t.Errorf("only %d fields checked", count)
	}
}

func TestSetListKeepsBlockStyle(t *testing.T) {
	t.Parallel()

	data := "---\nslug: ui\ntickets:\n  - AL-2\n  - AL-3\ntags: []\n---\n# UI\n"
	got, err := SetList([]byte(data), "tickets", []string{"AL-3", "AL-2", "AL-9"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "---\nslug: ui\ntickets:\n  - AL-3\n  - AL-2\n  - AL-9\ntags: []\n---\n# UI\n"; string(got) != want {
		t.Errorf("SetList(block) =\n%q\nwant\n%q", got, want)
	}
}

func TestSetListWritesFlowLists(t *testing.T) {
	t.Parallel()

	got, err := SetList([]byte(ticket), "depends-on", []string{"AL-4", "needs, comma"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "\ndepends-on: [AL-4, \"needs, comma\"]\ntags: ui\n") {
		t.Errorf("SetList(flow) =\n%s", got)
	}
	got, err = SetList([]byte(ticket), "depends-on", nil)
	if err != nil || !strings.Contains(string(got), "\ndepends-on: []\n") {
		t.Errorf("SetList(empty) = %v\n%s", err, got)
	}
}
