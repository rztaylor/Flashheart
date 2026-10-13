package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// The callers a write tool meets (agent-protocol §7.1).
const (
	oneRun      = "one run"      // one hook session works here: it is the caller
	twoRuns     = "two runs"     // two hook sessions work here: ambiguous
	noRun       = "no run"       // no hook session: the server starts a run
	otherBranch = "other branch" // a hook session here on another branch
	otherHolder = "other holder" // the caller's ticket is held by another live session
)

// policyCase is one write tool: its arguments against ticket DM-1, the
// code it answers for each caller ("ok" for success) and how many history
// entries a successful call appends across the project (board-format
// §Activity): one for every write that changes a ticket's column, fields
// or criteria, none for notes, handoffs, reviews, files and questions.
type policyCase struct {
	tool    string
	args    func(e *env) map[string]any
	codes   map[string]string
	history int
}

func ticketArgs(extra map[string]any) func(*env) map[string]any {
	return func(*env) map[string]any {
		args := map[string]any{"ticket": "DM-1"}
		for key, value := range extra {
			args[key] = value
		}
		return args
	}
}

// codes is the documented outcome for a tool: required run, run when
// ambiguous, and whether it guards the ticket against another holder.
func codes(required, guarded bool, holder string) map[string]string {
	m := map[string]string{oneRun: "ok", twoRuns: "ambiguous_run", noRun: "ok", otherBranch: "ok", otherHolder: "ok"}
	if required {
		m[otherBranch] = "ambiguous_run"
	}
	if guarded {
		m[otherHolder] = "claimed"
	}
	if holder != "" {
		m[otherHolder] = holder
	}
	return m
}

// policyCases mirrors the run and claim policy table in agent-protocol
// §7.1: every write tool, the code it answers for each caller.
var policyCases = []policyCase{
	{"claim", ticketArgs(nil), codes(true, false, "claimed"), 1},
	{"release", ticketArgs(nil), codes(true, false, ""), 0},
	{"ask_human", ticketArgs(map[string]any{"kind": "question", "text": "Which schema?"}), codes(true, false, ""), 0},
	{"checkpoint", ticketArgs(map[string]any{"done": []string{"Panel"}, "next": []string{"Tests"}}), codes(false, true, ""), 0},
	{"update_ticket", ticketArgs(map[string]any{"set": map[string]any{"priority": "high"}, "append_notes": "Checked."}), codes(false, true, ""), 1},
	{"move", ticketArgs(map[string]any{"to": "up-next"}), codes(false, true, ""), 1},
	{"write_review", ticketArgs(map[string]any{"markdown": "# Review: Card panel\n"}), codes(false, true, ""), 0},
	{"attach", func(e *env) map[string]any {
		file := filepath.Join(filepath.Dir(e.root), "run.log")
		if err := os.WriteFile(file, []byte("ok\n"), 0o644); err != nil {
			e.t.Fatal(err)
		}
		return map[string]any{"ticket": "DM-1", "path": file, "caption": "Test run"}
	}, codes(false, true, ""), 0},
	{"create_ticket", func(*env) map[string]any {
		return map[string]any{"type": "bug", "title": "Crash", "description": "It crashes.", "criteria": []string{"No crash"}, "priority": "high"}
	}, codes(false, false, ""), 1},
	{"create_workstream", func(*env) map[string]any {
		return map[string]any{"title": "Cards", "goal": "Card work", "tickets": []string{"DM-1"}}
	}, codes(false, true, ""), 1},
	{"set_project_key", func(*env) map[string]any { return map[string]any{"key": "DM"} }, codes(false, false, ""), 0},
}

// arrange sets up the caller: hook sessions as their hooks would record
// them, and another session's claim on DM-1.
func (e *env) arrange(caller string) {
	e.t.Helper()
	switch caller {
	case oneRun:
		e.startSession(session)
	case twoRuns:
		e.startSession(session)
		e.startSession(rival)
	case otherBranch:
		e.events(session, events.Event{Kind: events.RunStart, Data: events.RunStartData{Kind: events.KindSession, Cwd: e.cwd, Branch: "main", Worktree: e.cwd, Source: "startup"}})
	case otherHolder:
		e.rivalHolds("DM-1")
		e.startSession(session)
	}
}

// Every write tool declares its run and claim policy; this table is the
// contract of agent-protocol §7.1 and §7.2 for each tool and caller.
func TestWriteToolPolicies(t *testing.T) {
	t.Parallel()

	for _, tc := range policyCases {
		for _, caller := range []string{oneRun, twoRuns, noRun, otherBranch, otherHolder} {
			t.Run(tc.tool+"/"+strings.ReplaceAll(caller, " ", "-"), func(t *testing.T) {
				t.Parallel()

				e := newEnv(t, "DM")
				e.ticket(store.NewTicket{Title: "Card panel"})
				e.arrange(caller)
				before := len(e.log())
				want := tc.codes[caller]
				text, failed := e.call(tc.tool, tc.args(e))
				if want == "ok" {
					if failed {
						t.Fatalf("%s as %s failed:\n%s", tc.tool, caller, text)
					}
				} else if !failed || !strings.HasPrefix(text, "error "+want+":") || !strings.Contains(text, "\nfix: ") {
					t.Fatalf("%s as %s = (failed %v)\n%s\nwant error %s", tc.tool, caller, failed, text, want)
				}
				history := e.history()
				switch {
				case want != "ok":
					// A refused call records nothing and starts no run.
					if after := len(e.log()); after != before {
						t.Fatalf("a refused %s recorded %v", tc.tool, e.log()[before:])
					}
					if len(history) != 0 {
						t.Fatalf("a refused %s wrote history %+v", tc.tool, history)
					}
				case caller == noRun:
					// The run the server started is the caller.
					if len(e.started()) != 1 {
						t.Fatalf("%s started %v", tc.tool, e.started())
					}
				}
				if want != "ok" {
					return
				}
				// Every ticket write appends one history entry naming the
				// caller: its run, the run started for it, or unknown.
				by := session
				switch caller {
				case noRun:
					by = e.started()[0].Run
				case otherBranch:
					by = "unknown"
				}
				if len(history) != tc.history {
					t.Fatalf("%s as %s wrote history %+v, want %d entries", tc.tool, caller, history, tc.history)
				}
				for _, entry := range history {
					if entry.By != by || len(entry.Changes) == 0 {
						t.Errorf("%s as %s: history %+v, want it by %s", tc.tool, caller, entry, by)
					}
				}
			})
		}
	}
}

// Every registered write tool has a row in the table.
func TestEveryWriteToolHasAPolicyCase(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	e.connect()
	tools, err := e.client.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, tc := range policyCases {
		covered[tc.tool] = true
	}
	for _, tool := range tools.Tools {
		if !covered[tool.Name] && !readTools[tool.Name] {
			t.Errorf("write tool %s has no policy case", tool.Name)
		}
	}
}

var readTools = map[string]bool{"board_context": true, "list_tickets": true, "get_ticket": true}
