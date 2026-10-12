package board

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

var templateNow = time.Date(2026, 10, 11, 9, 30, 0, 0, time.UTC)

func golden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "templates", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The templates write byte for byte what store wrote before board owned
// them (testdata/templates, captured 2026-10-11, FH-87).
func TestTemplatesMatchTheirGoldenFiles(t *testing.T) {
	t.Parallel()

	full := NewTicket{Title: "  Drag   and drop ", Type: "bug", Priority: "high", Status: "up-next", Workstream: "board-ui",
		DependsOn: []string{"FH-1", "FH-2"}, Tags: []string{"ui", "dnd"}, Session: "claude:abc",
		Description: "  Cards move.\n\nBetween columns.  ", Criteria: []string{"one", "  two  words ", ""}, PlanOrRepro: "Drag a card."}
	bare := NewTicket{Title: "Bare"}
	for name, ticket := range map[string]struct {
		id string
		t  *NewTicket
	}{"ticket-full.md": {"FH-9", &full}, "ticket-bare.md": {"FH-10", &bare}} {
		if err := ticket.t.Normalize(); err != nil {
			t.Fatal(err)
		}
		data, err := RenderTicket(ticket.id, *ticket.t, templateNow)
		if err != nil || string(data) != golden(t, name) {
			t.Errorf("%s: %v\n%s", name, err, data)
		}
	}
	workstreams := map[string]struct {
		slug string
		w    NewWorkstream
	}{
		"workstream-full.md": {"board-ui", NewWorkstream{Title: "Board UI", Priority: "high", Goal: " Ship it. ", Tickets: []string{"FH-1", "FH-2"}, DependsOnWorkstreams: []string{"core"}, Tags: []string{"ui"}}},
		"workstream-bare.md": {"bare", NewWorkstream{Title: "Bare", Priority: "medium"}},
	}
	for name, w := range workstreams {
		data, err := RenderWorkstream(w.slug, w.w, templateNow)
		if err != nil || string(data) != golden(t, name) {
			t.Errorf("%s: %v\n%s", name, err, data)
		}
	}
}

func TestNewTicketsAreNormalized(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		ticket NewTicket
		err    string
	}{
		{NewTicket{Title: "  "}, "a ticket needs a title"},
		{NewTicket{Title: "x", Type: "epic"}, "type must be one of"},
		{NewTicket{Title: "x", Priority: "urgent"}, "priority must be one of"},
		{NewTicket{Title: "x", Status: "doing"}, `status "doing" is not a column`},
		{NewTicket{Title: "x", Workstream: "Board-UI"}, `workstream "Board-UI"`},
		{NewTicket{Title: "x", DependsOn: []string{"nope"}}, `depends-on "nope" is not a ticket id`},
	} {
		if err := tc.ticket.Normalize(); err == nil || !strings.Contains(err.Error(), tc.err) {
			t.Errorf("%+v: %v, want %q", tc.ticket, err, tc.err)
		}
	}
	w := NewWorkstream{Title: "x", Tickets: []string{"FH-1", "FH-1"}}
	if err := w.Normalize(); err == nil || !strings.Contains(err.Error(), "listed twice") {
		t.Errorf("duplicate: %v", err)
	}
}

func TestHandoffsRoundTrip(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 10, 11, 20, 51, 0, 0, time.UTC)
	for _, h := range []HandoffNote{
		{Updated: at, By: "claude:887c783f", Branch: "feature/x", Done: []string{"one", "two"}, Next: []string{"three"}, Files: []string{"a.go"}, Questions: []string{"why?"}},
		{Updated: at, By: "codex:mcp-9bd2", Next: []string{"only next"}},
		{Updated: at, By: "claude:1", Done: []string{"## Handoff", "**Next**"}, Next: []string{"## Notes and more"}},
		{Updated: at, By: "claude:2", Branch: "main", Done: []string{"text with ## Handoff inside"}},
	} {
		content := RenderHandoff(h)
		if got := ParseHandoff(content); !reflect.DeepEqual(got, h) {
			t.Errorf("round trip:\n%+v\n%+v\n%s", h, got, content)
		}
	}
	// Empty Files and Questions are written as None.
	content := RenderHandoff(HandoffNote{Updated: at, By: "claude:1", Next: []string{"x"}})
	if !strings.Contains(content, "**Files**\n\n- None\n") || !strings.Contains(content, "**Open questions**\n\n- None\n") || strings.Contains(content, "**Done**") {
		t.Errorf("render:\n%s", content)
	}
	// A hand-written handoff has lists but no header.
	if got := ParseHandoff("**Next**\n\n- Ship it\n"); got.By != "" || !got.Updated.IsZero() || !reflect.DeepEqual(got.Next, []string{"Ship it"}) {
		t.Errorf("hand-written: %+v", got)
	}
}

// sectionLiteral matches a ticket or workstream section named by a literal.
var sectionLiteral = regexp.MustCompile(`"(## )?(Description|Acceptance Criteria|Test Plan|Reproduction|Context|Notes|Handoff|Goal|Success Criteria)"|mdfile\.\w+Section\w*\([^,]+, "|"## `)

// Only board names the sections of ticket and workstream files (FH-87).
func TestOnlyBoardNamesSections(t *testing.T) {
	t.Parallel()

	base := filepath.Join("..", "..")
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(base, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			// board names the sections; mdfile writes markdown's own syntax.
			if relative, _ := filepath.Rel(base, path); filepath.Dir(relative) == filepath.Join("internal", "board") || filepath.Dir(relative) == filepath.Join("internal", "mdfile") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for number, line := range strings.Split(string(data), "\n") {
				if sectionLiteral.MatchString(line) {
					t.Errorf("%s:%d names a section: %s", path, number+1, strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
