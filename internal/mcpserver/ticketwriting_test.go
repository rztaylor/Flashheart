package mcpserver

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Tickets are written for a person first (FH-113): the tools say so where
// agents write, technical detail has its own Context section, and a title
// that reads like code is saved with advice on improving it.

func TestCreateTicketKeepsContextUnderItsSection(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	e.startSession(session)
	e.ok("create_ticket", map[string]any{"type": "feature", "title": "Cards show who holds them", "description": "People can't tell who is working on a card.",
		"criteria": []string{"The holder shows on the card"}, "priority": "medium", "context": " The holder comes from `runs.View`; see internal/api/runs.go. "})
	contains(t, e.file("DM-1"), "## Context\n\nThe holder comes from `runs.View`; see internal/api/runs.go.\n\n## Notes")
	e.fails("create_ticket", map[string]any{"type": "feature", "title": "x", "description": "", "criteria": []string{}, "priority": "low", "context": strings.Repeat("x", 17<<10)}, "too_large")
}

func TestTitlesThatReadLikeCodeGetAdvice(t *testing.T) {
	t.Parallel()

	const code, long = "warning: The title reads like code", "warning: The title is "
	for _, tc := range []struct {
		title, want string
	}{
		{"Agents write tickets a person can read at a glance", ""},
		{"Card and Runs tab can disagree about why a session needs you", ""},
		{"Paste screenshots on macOS, iPhone and GitHub", ""},
		{"Card colours in light/dark themes follow Node.js docs", ""},
		{"Header overlaps", ""},
		{"Keep notes short, e.g. one line", ""},
		{"TestWatchFoldsAppendedEvents can check the run too early", code + ` ("TestWatchFoldsAppendedEvents")`},
		{"Fix liveBadge ordering", code + ` ("liveBadge")`},
		{"Split `TicketView` into a loader", code + ` ("` + "`TicketView`" + `")`},
		{"Tidy internal/api/runs.go helpers", code + ` ("internal/api/runs.go")`},
		{"Tidy the helpers in runs.go", code + ` ("runs.go")`},
		{"Default plan_or_repro to empty", code + ` ("plan_or_repro")`},
		{"Call SetTitle() once per edit", code + ` ("SetTitle()")`},
		{"Move board.RenderTicket into store", code + ` ("board.RenderTicket")`},
		{"A session that ends while its question is open keeps showing as needing you on the board", long + "88 characters"},
	} {
		e := newEnv(t, "DM")
		e.startSession(session)
		created := e.ok("create_ticket", map[string]any{"type": "feature", "title": tc.title, "description": "", "criteria": []string{}, "priority": "low"})
		renamed := e.ok("update_ticket", map[string]any{"ticket": "DM-1", "set": map[string]any{"title": tc.title}})
		for tool, out := range map[string]string{"create_ticket": created, "update_ticket": renamed} {
			if !strings.HasSuffix(strings.TrimSpace(out), "ok ticket=DM-1") && !strings.Contains(out, "ok ticket=DM-1 changed=title") {
				t.Errorf("%s(%q) did not succeed:\n%s", tool, tc.title, out)
			}
			switch {
			case tc.want == "" && strings.Contains(out, "warning:"):
				t.Errorf("%s(%q) warned:\n%s", tool, tc.title, out)
			case tc.want != "" && !strings.Contains(out, tc.want):
				t.Errorf("%s(%q) = \n%s\nwant %s", tool, tc.title, out, tc.want)
			}
		}
	}
	// update_ticket only judges a title it sets.
	e := newEnv(t, "DM")
	e.startSession(session)
	e.ok("create_ticket", map[string]any{"type": "feature", "title": "Fix liveBadge ordering", "description": "", "criteria": []string{}, "priority": "low"})
	if out := e.ok("update_ticket", map[string]any{"ticket": "DM-1", "set": map[string]any{"priority": "high"}}); strings.Contains(out, "warning:") {
		t.Errorf("a priority change warned about the title:\n%s", out)
	}
}

func TestTicketToolsAskForTicketsWrittenForAPerson(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	e.connect()
	tools, err := e.client.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	type schema struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	found := map[string]bool{}
	for _, tool := range tools.Tools {
		if tool.Name != "create_ticket" && tool.Name != "update_ticket" {
			continue
		}
		found[tool.Name] = true
		if !strings.Contains(tool.Description, "a person") || !strings.Contains(tool.Description, "Context") {
			t.Errorf("%s description = %q", tool.Name, tool.Description)
		}
		data, _ := json.Marshal(tool.InputSchema)
		var input schema
		if err := json.Unmarshal(data, &input); err != nil {
			t.Fatal(err)
		}
		if tool.Name != "create_ticket" {
			continue
		}
		for _, field := range []string{"title", "description", "context"} {
			if hint := input.Properties[field].Description; hint == "" {
				t.Errorf("create_ticket's %s has no hint: %s", field, data)
			}
		}
	}
	if fmt.Sprint(found) != "map[create_ticket:true update_ticket:true]" {
		t.Fatalf("tools = %v", found)
	}
}
