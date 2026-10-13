package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// Codex names the hook session in every MCP call's _meta (agent-protocol
// §5.3, FH-72), so its calls are attributed to that session's run (§7.1,
// FH-74). These tests replay what Codex recorded: hook events from
// testdata/hooks/codex and the _meta of its tools/call.

var codexFixtures = filepath.Join("..", "..", "testdata", "hooks", "codex")

const (
	// codexRun is the recorded session's run; codexSubagent its recorded
	// subagent's.
	codexRun      = "codex:019a7c00-0000-7000-8000-000000000001"
	codexSubagent = codexRun + "/019a7c00-0000-7000-8000-000000000002"
	// recordedSession is the session id in the recordings, and
	// recordedCwd their working directory.
	recordedSession = "019a7c00-0000-7000-8000-000000000001"
	recordedCwd     = "/Users/example/src/demo"
)

// codexMeta is the _meta of a recorded Codex tools/call, naming session
// and thread instead of the recorded ones when they differ.
func codexMeta(t testing.TB, session, thread string) mcp.Meta {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(codexFixtures, "mcp", "tools-call-flashheart.json"))
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), recordedSession+":0", thread+":0"))
	var call struct {
		Params struct {
			Meta mcp.Meta `json:"_meta"`
		} `json:"params"`
	}
	if err := json.Unmarshal(data, &call); err != nil {
		t.Fatal(err)
	}
	meta := call.Params.Meta
	turn := meta["x-codex-turn-metadata"].(map[string]any)
	turn["session_id"], turn["thread_id"] = session, thread
	meta["sessionId"], meta["threadId"] = session, thread
	return meta
}

// codexHooks records what Codex's hooks recorded for payloads (like
// "SessionStart/startup"), in the env's checkout, at the env's time, for
// session instead of the recorded one.
func (e *env) codexHooks(session string, payloads ...string) {
	e.t.Helper()
	for _, payload := range payloads {
		data, err := os.ReadFile(filepath.Join(codexFixtures, payload+".events.json"))
		if err != nil {
			e.t.Fatal(err)
		}
		text := strings.ReplaceAll(string(data), recordedCwd, e.cwd)
		text = strings.ReplaceAll(text, "codex:"+recordedSession, "codex:"+session)
		var recorded []struct {
			Run  string          `json:"run"`
			Kind events.Kind     `json:"kind"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal([]byte(text), &recorded); err != nil {
			e.t.Fatalf("%s: %v", payload, err)
		}
		var list []events.Event
		for _, r := range recorded {
			var data any
			switch r.Kind {
			case events.RunStart:
				data = &events.RunStartData{}
			case events.TurnStart:
				data = &events.TurnStartData{}
			default:
				e.t.Fatalf("%s: add %s here", payload, r.Kind)
			}
			if err := json.Unmarshal(r.Data, data); err != nil {
				e.t.Fatal(err)
			}
			list = append(list, events.Event{Time: e.now, Run: r.Run, Agent: "codex", Kind: r.Kind, Project: "demo", Data: reflect.ValueOf(data).Elem().Interface()})
		}
		if err := events.New(e.store).Append(list...); err != nil {
			e.t.Fatal(err)
		}
	}
}

// callAs runs a tool over a client's connection with the call's _meta.
func (e *env) callAs(client *mcp.ClientSession, meta mcp.Meta, tool string, args map[string]any) (string, bool) {
	e.t.Helper()
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Meta: meta, Name: tool, Arguments: args})
	if err != nil {
		e.t.Fatalf("%s: protocol error: %v", tool, err)
	}
	var text strings.Builder
	for _, content := range result.Content {
		if c, ok := content.(*mcp.TextContent); ok {
			text.WriteString(c.Text)
		}
	}
	return text.String(), result.IsError
}

func (e *env) okAs(client *mcp.ClientSession, meta mcp.Meta, tool string, args map[string]any) string {
	e.t.Helper()
	text, failed := e.callAs(client, meta, tool, args)
	if failed {
		e.t.Fatalf("%s(%v) failed:\n%s", tool, args, text)
	}
	return text
}

// runsOf lists the runs the log names, in first-seen order.
func (e *env) runsOf() []string {
	var list []string
	for _, ev := range e.log() {
		if !slices.Contains(list, ev.Run) {
			list = append(list, ev.Run)
		}
	}
	return list
}

// With Codex's hooks, claim, checkpoint and ask_human land on the
// session's hook run, and the server starts none: not even beside another
// Codex session in the worktree, which leaves the worktree rule ambiguous.
func TestCodexCallsLandOnTheirHookRun(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	const other = "019a7c00-0000-7000-8000-0000000000ff"
	e.codexHooks(recordedSession, "SessionStart/startup", "UserPromptSubmit/prompt")
	e.codexHooks(other, "SessionStart/app-startup", "UserPromptSubmit/app-prompt")
	codex := e.dial(codexClient)
	meta := codexMeta(t, recordedSession, recordedSession)
	before := len(e.log())

	contains(t, e.okAs(codex, meta, "board_context", nil), "run=codex:00000001")
	contains(t, e.okAs(codex, meta, "claim", map[string]any{"ticket": id}), "ok ticket=DM-1")
	contains(t, e.okAs(codex, meta, "checkpoint", map[string]any{"ticket": id, "done": []string{"Panel"}, "next": []string{"Tests"}}), "by=codex:00000001")
	contains(t, e.okAs(codex, meta, "ask_human", map[string]any{"ticket": id, "kind": "question", "text": "Which schema?"}), "ok question=q-")

	if len(e.started()) != 0 {
		t.Fatalf("started %v beside the hook run", e.started())
	}
	var kinds []events.Kind
	for _, ev := range e.log()[before:] {
		if ev.Run != codexRun {
			t.Fatalf("event not attributed to the hook run: %+v", ev)
		}
		kinds = append(kinds, ev.Kind)
	}
	// Hooks count the session's tool calls, so the server records no
	// activity for it.
	if !slices.Equal(kinds, []events.Kind{events.Claim, events.TicketMoved, events.Checkpoint, events.QuestionAsked}) {
		t.Fatalf("events = %v", kinds)
	}
	set := e.fold()
	if r := set.Get(codexRun); r.Claim != id || len(r.Questions) != 1 || r.MCPOnly() {
		t.Fatalf("hook run = claim %q, %d questions, MCP-only %v", r.Claim, len(r.Questions), r.MCPOnly())
	}
}

// Two Codex sessions in one worktree each get their own calls, and a
// client that does not name its session is told which runs it could be.
func TestTwoCodexSessionsInOneWorktree(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	first := e.ticket(store.NewTicket{Title: "First"})
	second := e.ticket(store.NewTicket{Title: "Second"})
	const other = "019a7c00-0000-7000-8000-0000000000ff"
	e.codexHooks(recordedSession, "SessionStart/startup", "UserPromptSubmit/prompt")
	e.codexHooks(other, "SessionStart/startup", "UserPromptSubmit/prompt")
	one, two := e.dial(codexClient), e.dial(codexClient)

	e.okAs(one, codexMeta(t, recordedSession, recordedSession), "claim", map[string]any{"ticket": first})
	e.okAs(two, codexMeta(t, other, other), "claim", map[string]any{"ticket": second})
	e.okAs(two, codexMeta(t, other, other), "ask_human", map[string]any{"kind": "question", "text": "Two?"})
	set := e.fold()
	mine, theirs := set.Get(codexRun), set.Get("codex:"+other)
	if mine.Claim != first || theirs.Claim != second || len(mine.Questions) != 0 || len(theirs.Questions) != 1 {
		t.Fatalf("claims %q, %q; questions %d, %d", mine.Claim, theirs.Claim, len(mine.Questions), len(theirs.Questions))
	}
	if len(e.started()) != 0 {
		t.Fatalf("started %v", e.started())
	}

	// Without _meta (an older Codex), the worktree rule cannot choose.
	out, failed := e.callOn(e.dial(codexClient), "checkpoint", map[string]any{"ticket": first, "done": []string{"x"}, "next": []string{"y"}})
	if !failed || !strings.HasPrefix(out, "error ambiguous_run:") {
		t.Fatalf("checkpoint without _meta:\n%s", out)
	}
	// Codex's session ids start with the time, so two sessions started in
	// the same minute are named by their ends (FH-122), and each name
	// picks out its session.
	contains(t, out, "fix: pass run", "2 sessions", "codex:00000001, codex:000000ff")
	contains(t, e.okOn(e.dial(codexClient), "board_context", map[string]any{"run": "codex:000000ff"}), "run=codex:000000ff")
}

// Two Codex sessions started in the same minute have their own short ids:
// the board shows them apart, and the run id each recovery note gives
// names only that session (FH-122).
func TestCodexSessionsStartedTogetherHaveTheirOwnShortIDs(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	const other = "019a7c00-0000-7000-8000-0000000000ff"
	e.codexHooks(recordedSession, "SessionStart/startup", "UserPromptSubmit/prompt")
	e.codexHooks(other, "SessionStart/startup", "UserPromptSubmit/prompt")

	if mine, theirs := protocol.ShortRun(codexRun), protocol.ShortRun("codex:"+other); mine != "codex:00000001" || theirs != "codex:000000ff" {
		t.Fatalf("short ids %q and %q", mine, theirs)
	}
	contains(t, e.ok("board_context", map[string]any{"run": "codex:00000001"}), "run=codex:00000001")
	contains(t, e.ok("claim", map[string]any{"ticket": id, "run": "codex:000000ff"}), "ok ticket=DM-1")
	if holder := e.fold().Get("codex:" + other); holder.Claim != id {
		t.Fatalf("claim landed on %+v", holder)
	}
	// The start of the id, as short ids were before, still names both.
	e.fails("board_context", map[string]any{"run": "codex:019a7c00"}, "ambiguous_run")
}

// A Codex session without hooks gets a run under its own session id; when
// its hooks start mid-session they join that run, so the session keeps one
// claim holder and one Needs you entry, and the server stops standing in
// for its hooks.
func TestCodexHooksThatStartMidSessionJoinItsRun(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Card panel"})
	codex := e.dial(codexClient)
	meta := codexMeta(t, recordedSession, recordedSession)

	contains(t, e.okAs(codex, meta, "board_context", nil), "run=unknown")
	if len(e.log()) != 0 {
		t.Fatalf("a read recorded %v", e.log())
	}
	e.okAs(codex, meta, "claim", map[string]any{"ticket": id})
	e.okAs(codex, meta, "ask_human", map[string]any{"ticket": id, "kind": "question", "text": "Which schema?"})
	list := e.started()
	if len(list) != 1 || list[0].Run != codexRun {
		t.Fatalf("started = %v, want %s", list, codexRun)
	}
	set := e.fold()
	if r := set.Get(codexRun); !r.MCPOnly() || r.Claim != id || set.State(codexRun, e.now, runs.DefaultSettings()) != runs.NeedsYou {
		t.Fatalf("hookless run = %+v", r)
	}

	// The user trusts the hooks; the next prompt reports the session.
	e.now = e.now.Add(2 * time.Minute)
	e.codexHooks(recordedSession, "UserPromptSubmit/prompt")
	before := len(e.log())
	e.now = e.now.Add(2 * time.Minute)
	contains(t, e.okAs(codex, meta, "checkpoint", map[string]any{"ticket": id, "done": []string{"Panel"}, "next": []string{"Tests"}}), "by=codex:00000001")
	var kinds []events.Kind
	for _, ev := range e.log()[before:] {
		kinds = append(kinds, ev.Kind)
	}
	// The activity the server counted before the hooks is recorded; from
	// now on the hooks count the session's tool calls.
	if !slices.Equal(kinds, []events.Kind{events.Activity, events.Checkpoint}) {
		t.Fatalf("events after the hooks joined = %v", kinds)
	}
	e.okAs(codex, meta, "board_context", nil)
	if got := e.log()[len(e.log())-1]; got.Kind != events.Checkpoint {
		t.Fatalf("the server still records the run's activity: %+v", got)
	}
	if ids := e.runsOf(); !slices.Equal(ids, []string{codexRun}) {
		t.Fatalf("runs = %v", ids)
	}
	set = e.fold()
	r := set.Get(codexRun)
	if r.MCPOnly() || r.Claim != id || len(r.Questions) != 1 {
		t.Fatalf("joined run = MCP-only %v, claim %q, %d questions", r.MCPOnly(), r.Claim, len(r.Questions))
	}
	if holder := set.Holder(id, e.now, runs.DefaultSettings()); holder == nil || holder.ID != codexRun {
		t.Fatalf("holder = %+v", holder)
	}

	// Closing the connection leaves the end to the session's hooks.
	if err := codex.Close(); err != nil {
		t.Fatal(err)
	}
	e.srv.ending.Wait()
	if kinds := e.kinds(codexRun); slices.Contains(kinds, events.RunEnd) {
		t.Fatalf("the server ended a run hooks report: %v", kinds)
	}
}

// A hookless Codex session's run takes answers in its tool results and
// ends when its connection closes, as any run the server starts.
func TestAHooklessCodexRunIsTheServersToEnd(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	codex := e.dial(codexClient)
	meta := codexMeta(t, recordedSession, recordedSession)
	e.okAs(codex, meta, "ask_human", map[string]any{"kind": "question", "text": "Which schema?"})
	if err := codex.Close(); err != nil {
		t.Fatal(err)
	}
	e.srv.ending.Wait()
	kinds := e.kinds(codexRun)
	if len(kinds) == 0 || kinds[len(kinds)-1] != events.RunEnd {
		t.Fatalf("events = %v", kinds)
	}
}

// A call from a Codex subagent is its subagent run's when its thread is a
// subagent the hooks reported, else its session's (whether Codex names
// the subagent's thread in _meta is not recorded).
func TestACodexSubagentsCallIsItsRun(t *testing.T) {
	t.Parallel()

	const subagent = "019a7c00-0000-7000-8000-000000000002"
	for _, tc := range []struct {
		name            string
		session, thread string
		want            string
	}{
		{"the session's thread", recordedSession, recordedSession, codexRun},
		{"a subagent's thread", recordedSession, subagent, codexSubagent},
		{"a subagent named as its own session", subagent, subagent, codexSubagent},
		{"a thread hooks never reported", recordedSession, "019a7c00-0000-7000-8000-0000000000ee", codexRun},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, "DM")
			id := e.ticket(store.NewTicket{Title: "Card panel"})
			e.codexHooks(recordedSession, "SessionStart/startup", "UserPromptSubmit/prompt", "SubagentStart/default")
			codex := e.dial(codexClient)
			e.okAs(codex, codexMeta(t, tc.session, tc.thread), "update_ticket", map[string]any{"ticket": id, "append_notes": "Checked."})
			last := e.log()[len(e.log())-1]
			if last.Kind != events.TicketUpdated || last.Run != tc.want || len(e.started()) != 0 {
				t.Fatalf("last event %+v, started %v", last, e.started())
			}
		})
	}
}

// Only Codex's calls name a session: one carrying Codex's turn metadata,
// or from a client named as Codex; a session that is no run id is none.
func TestCodexSessionFromMeta(t *testing.T) {
	t.Parallel()

	recorded := codexMeta(t, recordedSession, recordedSession)
	codex, claude := &conn{agent: codexAgent}, &conn{agent: "claude"}
	for _, tc := range []struct {
		name            string
		meta            mcp.Meta
		cn              *conn
		session, thread string
	}{
		{"a recorded call", recorded, codex, recordedSession, recordedSession},
		{"turn metadata from any client", recorded, claude, recordedSession, recordedSession},
		{"turn metadata without a connection", recorded, nil, recordedSession, recordedSession},
		{"sessionId alone from Codex", mcp.Meta{"sessionId": "s-1", "threadId": "t-1"}, codex, "s-1", "t-1"},
		{"sessionId alone from another client", mcp.Meta{"sessionId": "s-1"}, claude, "", ""},
		{"turn metadata wins", mcp.Meta{"sessionId": "s-1", "x-codex-turn-metadata": map[string]any{"session_id": "s-2"}}, codex, "s-2", ""},
		{"a session that is no run id", mcp.Meta{"sessionId": "../etc"}, codex, "", ""},
		{"a thread that is no run id", mcp.Meta{"sessionId": "s-1", "threadId": "a b"}, codex, "s-1", ""},
		{"no _meta", nil, codex, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if session, thread := codexSession(tc.meta, tc.cn); session != tc.session || thread != tc.thread {
				t.Fatalf("codexSession = %q, %q; want %q, %q", session, thread, tc.session, tc.thread)
			}
		})
	}
}
