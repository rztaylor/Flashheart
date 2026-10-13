package hooks

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/store"
)

var now = time.Date(2026, 10, 5, 14, 12, 9, 0, time.UTC)

// fake is a minimal adapter: the payload is "<session> <cwd>" and every
// event becomes a turn start, or a session start for "SessionStart".
type fake struct{ panicOn string }

func (f fake) Agent() string { return "fake" }

func (f fake) Parse(event string, payload []byte) (Input, error) {
	if event == f.panicOn {
		panic("adapter bug")
	}
	session, cwd, ok := strings.Cut(strings.TrimSpace(string(payload)), " ")
	if !ok {
		return Input{}, errors.New("malformed payload")
	}
	input := Input{Cwd: cwd}
	run := "fake:" + session
	// "Sub:<event>" is a tool event from inside the session's subagent a1.
	actor := run
	if rest, sub := strings.CutPrefix(event, "Sub:"); sub {
		event, actor = rest, run+"/a1"
	}
	tool := func(name string, ok bool, paths ...string) []Pending {
		return []Pending{{Run: actor, Kind: events.ToolUsed, Data: events.ToolData{Tool: name, OK: ok, Paths: paths}}}
	}
	// The fake's shell tool has no Claude Code name: hooks knows a shell
	// command only from the adapter's classification (FH-90).
	shell := func(ok bool) []Pending {
		return []Pending{{Run: actor, Kind: events.ToolUsed, Data: events.ToolData{Tool: "exec_command", OK: ok, Shell: true}}}
	}
	// "Edit:<file>" edits a file of the checkout; "Patch:<file>,<file>…"
	// edits several in one result.
	if file, edit := strings.CutPrefix(event, "Edit:"); edit {
		input.Events = tool("Edit", true, filepath.Join(cwd, file))
		return input, nil
	}
	if files, patch := strings.CutPrefix(event, "Patch:"); patch {
		var paths []string
		for _, file := range strings.Split(files, ",") {
			paths = append(paths, filepath.Join(cwd, file))
		}
		input.Events = tool("apply_patch", true, paths...)
		return input, nil
	}
	switch event {
	case "Read":
		input.Events = tool("Read", true)
	case "Fail":
		input.Events = shell(false)
	case "Checkpointed":
		// The checkpoint tool's own result (agent-protocol §5.2).
		input.Events, input.Settled = tool("mcp__flashheart__checkpoint", true), true
	case "Background":
		// A turn the agent's own background task started, not a prompt.
		input.Events = []Pending{{Run: run, Kind: events.TurnStart, Data: events.TurnStartData{Background: true}}}
	case "Agent":
		// A turn another agent's message started (FH-57), told apart by its
		// origin alone.
		input.Events = []Pending{{Run: run, Kind: events.TurnStart, Data: events.TurnStartData{Origin: events.OriginAgent}}}
	case "Permission":
		input.Events = []Pending{{Run: actor, Kind: events.PermissionRequested, Data: events.PermissionData{Tool: "Bash"}}}
	case "SubagentStart":
		input.Events = []Pending{{Run: run + "/a1", Kind: events.RunStart, Data: events.RunStartData{Kind: events.KindSubagent, Parent: run}}}
	case "SubagentStop":
		input.Events = []Pending{{Run: run + "/a1", Kind: events.RunEnd, Data: events.RunEndData{Reason: "completed"}}}
	case "SessionStart":
		input.Recovery = true
		input.Events = []Pending{{Run: run, Kind: events.RunStart, Data: events.RunStartData{Kind: events.KindSession, Source: "startup"}}}
	case "Edit":
		input.Events = tool("Edit", true, filepath.Join(cwd, "src", "a.ts"))
	case "Bash":
		input.Events = shell(true)
	case "End":
		input.Events = []Pending{{Run: run, Kind: events.RunEnd, Data: events.RunEndData{Reason: "other"}}}
	case "Plan":
		input.Events = []Pending{{Run: run, Kind: events.PlanUpdated, Data: events.PlanData{Items: []events.PlanItem{{Text: "Rotate token=abcdef123456\nnow", Status: "pending"}}}}}
	case "Prompt":
		input.Inbox = run
		input.Events = []Pending{{Run: run, Kind: events.TurnStart, Data: events.TurnStartData{}}}
	case "Stop", "StopActive":
		input.Stop, input.StopActive = true, event == "StopActive"
		input.Events = []Pending{{Run: run, Kind: events.TurnEnd, Data: events.TurnEndData{}}}
	case "Foreign", "Malformed":
		// An adapter bug: a run of another agent, or outside the grammar,
		// beside a good event (FH-91).
		bad := "other:" + session
		if event == "Malformed" {
			bad = "fake:" + session + " x"
		}
		input.Events = []Pending{
			{Run: bad, Kind: events.TurnStart, Data: events.TurnStartData{}},
			{Run: run, Kind: events.TurnStart, Data: events.TurnStartData{}},
		}
	case "Stamp":
		input.Reply = &Output{Context: "stamped " + run}
	case "Nothing":
	default:
		input.Events = []Pending{{Run: run, Kind: events.TurnStart, Data: events.TurnStartData{}}}
	}
	return input, nil
}

func (f fake) Render(event string, out Output) []byte {
	switch {
	case out.Block != "":
		return []byte("block:" + out.Block)
	case out.Context == "":
		return nil
	}
	return []byte("context:" + out.Context)
}

// repo makes a git checkout named demo on branch feature/demo.
func repo(t *testing.T) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "src", "demo")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/feature/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func run(t *testing.T, root, event, payload string, adapter Adapter) string {
	t.Helper()
	var stdout bytes.Buffer
	// A generous worktree check deadline: parallel tests load the machine.
	Run(Options{Root: root, Event: event, Stdin: strings.NewReader(payload), Stdout: &stdout, Now: func() time.Time { return now }, Adapter: adapter, ChangeTimeout: 10 * time.Second})
	return stdout.String()
}

func readEvents(t *testing.T, root, project string) []events.Event {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var list []events.Event
	if err := events.New(s).Read(project, time.Time{}, func(e events.Event) { list = append(list, e) }); err != nil {
		t.Fatal(err)
	}
	return list
}

func errorLog(t *testing.T, root string) string {
	t.Helper()
	data, _ := os.ReadFile(filepath.Join(root, ".flashheart", "hook-errors.log"))
	return string(data)
}

func TestRunAppendsEventsAndCreatesTheProject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cwd := repo(t)
	if out := run(t, root, "UserPromptSubmit", "s1 "+cwd, fake{}); out != "" {
		t.Fatalf("stdout = %q", out)
	}
	run(t, root, "Edit", "s1 "+cwd, fake{})
	run(t, root, "Plan", "s1 "+cwd, fake{})

	data, err := os.ReadFile(filepath.Join(root, "demo", "project.yaml"))
	if err != nil || !strings.Contains(string(data), "repos:\n  - "+cwd) || strings.Contains(string(data), "key:") {
		t.Fatalf("project.yaml = %q, %v", data, err)
	}
	list := readEvents(t, root, "demo")
	if len(list) != 3 {
		t.Fatalf("events = %+v", list)
	}
	var turn events.TurnStartData
	_ = list[0].Decode(&turn)
	if list[0].Run != "fake:s1" || list[0].Agent != "fake" || list[0].Project != "demo" || !list[0].Time.Equal(now) {
		t.Fatalf("envelope = %+v", list[0])
	}
	if turn.Cwd != cwd || turn.Branch != "feature/demo" || turn.Worktree != cwd {
		t.Fatalf("turn data = %+v", turn)
	}
	var edit events.ActivityData
	_ = list[1].Decode(&edit)
	if list[1].Kind != events.Activity || edit.Tools != 1 || len(edit.Paths) != 1 || edit.Paths[0] != "src/a.ts" {
		t.Fatalf("edit = %s %+v, want an activity record with the repository-relative path", list[1].Kind, edit)
	}
	var plan events.PlanData
	_ = list[2].Decode(&plan)
	if plan.Items[0].Text != "Rotate token=[redacted] now" {
		t.Fatalf("plan text = %q, want scrubbed single line", plan.Items[0].Text)
	}
	if _, err := os.Stat(filepath.Join(root, ".flashheart", "cache", "cwd.json")); err != nil {
		t.Fatalf("cwd cache not written: %v", err)
	}
	if log := errorLog(t, root); log != "" {
		t.Fatalf("hook-errors.log = %q", log)
	}
}

func TestRunFailsOpen(t *testing.T) {
	t.Parallel()

	t.Run("malformed input is logged", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if out := run(t, root, "UserPromptSubmit", "garbage", fake{}); out != "" {
			t.Fatalf("stdout = %q", out)
		}
		if log := errorLog(t, root); !strings.Contains(log, "fake UserPromptSubmit: malformed payload") {
			t.Fatalf("hook-errors.log = %q", log)
		}
	})
	t.Run("an adapter panic is logged", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		run(t, root, "Boom", "s1 /tmp", fake{panicOn: "Boom"})
		if log := errorLog(t, root); !strings.Contains(log, "panic: adapter bug") {
			t.Fatalf("hook-errors.log = %q", log)
		}
	})
	t.Run("oversized input is logged and not parsed", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		run(t, root, "UserPromptSubmit", "s1 "+repo(t)+strings.Repeat(" ", MaxPayloadBytes), fake{})
		if log := errorLog(t, root); !strings.Contains(log, "larger than") {
			t.Fatalf("hook-errors.log = %q", log)
		}
		if _, err := os.Stat(filepath.Join(root, "demo")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("oversized payload created a project")
		}
	})
	t.Run("a root that is a file does not panic", func(t *testing.T) {
		t.Parallel()
		root := filepath.Join(t.TempDir(), "root")
		if err := os.WriteFile(root, []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		if out := run(t, root, "UserPromptSubmit", "s1 "+repo(t), fake{}); out != "" {
			t.Fatalf("stdout = %q", out)
		}
	})
	t.Run("a corrupt config is logged and nothing is written", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".flashheart"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".flashheart", "config.yaml"), []byte("quiet_minutes: [\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		run(t, root, "UserPromptSubmit", "s1 "+repo(t), fake{})
		if log := errorLog(t, root); !strings.Contains(log, "config.yaml") {
			t.Fatalf("hook-errors.log = %q", log)
		}
		if _, err := os.Stat(filepath.Join(root, "demo")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("wrote to a root with a corrupt config")
		}
	})
	t.Run("missing permissions are logged", func(t *testing.T) {
		t.Parallel()
		if os.Geteuid() == 0 {
			t.Skip("root ignores permissions")
		}
		root := t.TempDir()
		cwd := repo(t)
		run(t, root, "UserPromptSubmit", "s1 "+cwd, fake{})
		events := filepath.Join(root, "demo", ".flashheart", "events")
		if err := os.Chmod(events, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(events, 0o755) })
		// A new day's file cannot be created in the read-only directory.
		var stdout bytes.Buffer
		Run(Options{Root: root, Event: "UserPromptSubmit", Stdin: strings.NewReader("s1 " + cwd), Stdout: &stdout, Now: func() time.Time { return now.Add(24 * time.Hour) }, Adapter: fake{}})
		if log := errorLog(t, root); !strings.Contains(log, "permission denied") {
			t.Fatalf("hook-errors.log = %q", log)
		}
	})
}

func TestRunWithoutAutoCreateSkipsUnknownProjects(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".flashheart"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".flashheart", "config.yaml"), []byte("auto_create_projects: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "UserPromptSubmit", "s1 "+repo(t), fake{})
	if _, err := os.Stat(filepath.Join(root, "demo")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("project created with auto_create_projects off")
	}
	if log := errorLog(t, root); log != "" {
		t.Fatalf("hook-errors.log = %q", log)
	}
}

// Outside git nothing is recorded (PRJ-4): a whole session, down to a lone
// run.end, creates no project, no cache and not even the root, and prints
// nothing.
func TestEventsOutsideGitRecordNothing(t *testing.T) {
	t.Parallel()

	outside, _ := filepath.EvalSymlinks(t.TempDir())
	for _, script := range [][]string{{"SessionStart", "Prompt", "Edit", "Bash", "Stop", "End"}, {"End"}} {
		root := filepath.Join(t.TempDir(), "absent")
		for _, event := range script {
			if out := run(t, root, event, "s1 "+outside, fake{}); out != "" {
				t.Errorf("%s output = %q", event, out)
			}
		}
		if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%v outside git created the root: %v", script, err)
		}
	}
	// With the root there, it is left as it was.
	root := t.TempDir()
	run(t, root, "SessionStart", "s1 "+outside, fake{})
	run(t, root, "End", "s1 "+outside, fake{})
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatalf("root after a session outside git = %v", entries)
	}
}

func TestNothingToRecordTouchesNothing(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "absent")
	run(t, root, "Nothing", "s1 "+repo(t), fake{})
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a hook with nothing to record created the root")
	}
}

func writeTicket(t *testing.T, root, project, folder, content string) {
	t.Helper()
	dir := filepath.Join(root, project, "tickets", folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, folder+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSessionStartReturnsTheRecoveryNote(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cwd := repo(t)
	// No linked ticket and no earlier run: no note.
	if out := run(t, root, "SessionStart", "s0 "+cwd, fake{}); out != "" {
		t.Fatalf("first session output = %q", out)
	}
	if err := os.WriteFile(filepath.Join(root, "demo", "project.yaml"), []byte("name: demo\nkey: DM\nnext_id: 2\nrepos:\n  - "+cwd+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTicket(t, root, "demo", "DM-1-card-panel", "---\nid: DM-1\nstatus: in-progress\ntype: feature\npriority: high\ncreated: 2026-10-01\nbranch: feature/demo\n---\n# Card panel\n\n## Handoff\n\n**Next**\n\n- Wire the Runs tab\n")
	// An earlier session edited files and ended without a checkpoint.
	run(t, root, "Edit", "s0 "+cwd, fake{})

	out := run(t, root, "SessionStart", "s1 "+cwd, fake{})
	for _, part := range []string{
		"context:[Flashheart] run=fake:s1 project=demo (key DM) branch=feature/demo",
		`Ticket DM-1 "Card panel" (in-progress, linked by branch). Previous run fake:s0 in this worktree is still open, 1 edit since its last checkpoint.`,
		"Last handoff — Next: Wire the Runs tab.",
		"Use the flashheart MCP tools: claim to continue, checkpoint before you stop.",
	} {
		if !strings.Contains(out, part) {
			t.Fatalf("output missing %q:\n%s", part, out)
		}
	}
}

func TestV1ProjectsAreLeftForMigrate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "demo", "todo"), 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, root, "UserPromptSubmit", "s1 "+repo(t), fake{})
	entries, _ := os.ReadDir(filepath.Join(root, "demo"))
	if len(entries) != 1 || entries[0].Name() != "todo" {
		t.Fatalf("v1 project now holds %v", entries)
	}
	if log := errorLog(t, root); log != "" {
		t.Fatalf("hook-errors.log = %q", log)
	}
}

func TestArchivedProjectsRepositoryIsQuiet(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cwd := repo(t)
	run(t, root, "UserPromptSubmit", "s1 "+cwd, fake{})
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ArchiveProject("demo"); err != nil {
		t.Fatal(err)
	}
	s.Close()

	for _, event := range []string{"SessionStart", "UserPromptSubmit", "Edit", "Stop"} {
		if out := run(t, root, event, "s1 "+cwd, fake{}); out != "" {
			t.Errorf("%s stdout = %q, want nothing", event, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "demo")); !errors.Is(err, os.ErrNotExist) {
		t.Error("the archived project was recreated")
	}
	if log := errorLog(t, root); log != "" {
		t.Errorf("hook-errors.log = %q", log)
	}
}

func TestSessionStartRefreshesTheProtocolText(t *testing.T) {
	t.Parallel()

	cwd := repo(t)
	start := func(root string, refresh func() (string, error)) string {
		var stdout bytes.Buffer
		Run(Options{Root: root, Event: "SessionStart", Stdin: strings.NewReader("s1 " + cwd), Stdout: &stdout, Now: func() time.Time { return now }, Adapter: fake{}, RefreshProtocol: refresh})
		return stdout.String()
	}

	root := t.TempDir()
	calls := 0
	refreshed := func() (string, error) { calls++; return protocol.SkillUpdatedNote, nil }
	if out := start(root, refreshed); !strings.Contains(out, "context:"+protocol.SkillUpdatedNote) {
		t.Fatalf("output = %q, want the refresher's note", out)
	}
	current := func() (string, error) { calls++; return "", nil }
	if out := start(root, current); out != "" {
		t.Fatalf("output = %q, want nothing for current protocol text", out)
	}
	// Only session start refreshes the protocol text.
	run(t, root, "Prompt", "s1 "+cwd, fake{})
	if calls != 2 {
		t.Fatalf("refresh called %d times, want 2", calls)
	}

	// A failed refresh is logged and the session start goes on.
	failed := func() (string, error) { return "", errors.New("skill: permission denied") }
	if out := start(root, failed); out != "" {
		t.Fatalf("output = %q", out)
	}
	if log := errorLog(t, root); !strings.Contains(log, "skill: permission denied") {
		t.Fatalf("hook-errors.log = %q", log)
	}
	if list := readEvents(t, root, "demo"); len(list) != 4 {
		t.Fatalf("events = %d, want 4 (every session start recorded)", len(list))
	}
}

// hooks drops, and logs, an event whose run id is malformed or names
// another agent, and records the rest (FH-91, HOOK-1).
func TestPendingEventsNeedTheAdaptersOwnRunIDs(t *testing.T) {
	t.Parallel()

	for _, event := range []string{"Foreign", "Malformed"} {
		t.Run(event, func(t *testing.T) {
			t.Parallel()
			root, cwd := t.TempDir(), repo(t)
			if out := run(t, root, event, "s1 "+cwd, fake{}); out != "" {
				t.Fatalf("output %q", out)
			}
			list := readEvents(t, root, "demo")
			if len(list) != 1 || list[0].Run != "fake:s1" {
				t.Fatalf("recorded %+v, want only the fake:s1 turn start", list)
			}
			if log := errorLog(t, root); !strings.Contains(log, "invalid run id") && !strings.Contains(log, "not a fake run") {
				t.Fatalf("hook-errors.log = %q", log)
			}
		})
	}
}
