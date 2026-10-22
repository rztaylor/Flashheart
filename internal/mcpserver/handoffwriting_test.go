package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/store"
)

// Handoffs are written for the person reviewing the ticket (FH-187): the
// checkpoint tool says so, and an item that runs long or reads like code is
// saved with advice on rewriting it.

func TestHandoffItemsThatReadLikeCodeGetAdvice(t *testing.T) {
	t.Parallel()

	long := "Overlap must match the rules the board already uses: names compare without case or surrounding space when both are given, versions drop one leading v and must be dotted numbers or count as open, compare number by number with missing parts as zero, and both ends of a range count as inside it."
	for _, tc := range []struct {
		name      string
		input     map[string]any
		want, not []string
	}{
		{"plain", map[string]any{"done": []string{"Facts now keep when they were true apart from when they were recorded"},
			"next":           []string{"Refuse a fact whose scope is malformed when it is added", "Run scripts/check.sh and open the pull request"},
			"open_questions": []string{"Should an old fact with no dates show as unknown or as always true?"}}, nil, []string{"warning:"}},
		{"names from the code", map[string]any{"done": []string{"Done"}, "next": []string{"Call Scope.Validate in contribute() before snapshot_decode runs"}},
			[]string{`warning: Next item 1 reads like code ("Scope.Validate", "contribute()", "snapshot_decode")`, "for the person reviewing the ticket", "Context"}, nil},
		{"too long", map[string]any{"done": []string{long}, "next": []string{"Open the pull request"}},
			[]string{"warning: Done item 1 is 292 characters", "a sentence or two", "Context"}, []string{"Next item"}},
		{"open question", map[string]any{"done": []string{"Done"}, "open_questions": []string{"Keep store.LifecycleActive or use domain.LifecycleActive with lifecycle_state?"}},
			[]string{"warning: Open question 1 reads like code"}, nil},
		{"one item each", map[string]any{"next": []string{"Plain step", "Fix liveBadge in runs.go via sortRuns()"}},
			[]string{"warning: Next item 2"}, []string{"Next item 1"}},
		{"a name once", map[string]any{"next": []string{"Default as_of to as_of_time when as_of is blank"}},
			[]string{`warning: Next item 1 reads like code ("as_of", "as_of_time")`}, nil},
	} {
		e := newEnv(t, "DM")
		id := e.ticket(store.NewTicket{Title: "Facts keep two clocks"})
		e.startSession(session)
		e.ok("claim", map[string]any{"ticket": id})
		tc.input["ticket"] = id
		out := e.ok("checkpoint", tc.input)
		contains(t, out, "ok ticket="+id+" checkpoint")
		for _, want := range tc.want {
			if !strings.Contains(out, want) {
				t.Errorf("%s: checkpoint said\n%s\nwant %q", tc.name, out, want)
			}
		}
		for _, not := range tc.not {
			if strings.Contains(out, not) {
				t.Errorf("%s: checkpoint said\n%s\nwant no %q", tc.name, out, not)
			}
		}
		// The handoff is saved either way.
		next, _ := tc.input["next"].([]string)
		for _, item := range next {
			contains(t, e.file(id), item)
		}
	}
}

func TestCheckpointAsksForAHandoffWrittenForAPerson(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	e.connect()
	tools, err := e.client.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "checkpoint" {
			continue
		}
		if !strings.Contains(tool.Description, "the person reviewing the ticket") || !strings.Contains(tool.Description, "Context") {
			t.Errorf("checkpoint description = %q", tool.Description)
		}
		data, _ := json.Marshal(tool.InputSchema)
		var input struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(data, &input); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"done", "next", "open_questions"} {
			if hint := input.Properties[field].Description; !strings.Contains(hint, "plain words") {
				t.Errorf("checkpoint's %s hint = %q", field, hint)
			}
		}
		return
	}
	t.Fatal("no checkpoint tool")
}
