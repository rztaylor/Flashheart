package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
)

// stateFile is the session's activity state (FH-55).
func stateFile(root string) string {
	return filepath.Join(root, "demo", ".flashheart", "activity", "fake--s1.json")
}

// recorded folds the demo project's log and returns the activity records
// with their data, and fails on any tool.used: hooks no longer write it.
func recorded(t *testing.T, root string) ([]events.Event, []events.ActivityData, *runs.Set) {
	t.Helper()
	set := runs.NewSet()
	var list []events.Event
	var data []events.ActivityData
	for _, e := range readEvents(t, root, "demo") {
		set.Apply(e)
		switch e.Kind {
		case events.ToolUsed:
			t.Fatalf("hooks wrote a tool.used: %+v", e)
		case events.Activity:
			var d events.ActivityData
			if err := e.Decode(&d); err != nil {
				t.Fatal(err)
			}
			list, data = append(list, e), append(data, d)
		}
	}
	return list, data, set
}

// FH-55: a run's tool results are recorded as at most one activity record a
// minute (activity_seconds), with every tool use and failure counted, plus
// one for each newly edited path and one at the turn end for what is left.
func TestToolResultsAreThrottledToActivityRecords(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		config string
		most   int
	}{
		// Ten minutes of tools at the default minute: ten records, the two
		// paths' and the turn end's.
		{"default interval", "", 10 + 2 + 1},
		{"five minute interval", "activity_seconds: 300\n", 2 + 2 + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, cwd := t.TempDir(), repo(t)
			if tc.config != "" {
				if err := os.MkdirAll(filepath.Join(root, ".flashheart"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".flashheart", "config.yaml"), []byte(tc.config), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			mix := []string{"Read", "Bash", "Read", "Fail", "Edit:src/a.ts", "Read", "Edit:src/b.ts", "Bash", "Edit:src/a.ts", "Read"}
			steps := []string{"0 SessionStart", "0 Prompt"}
			failures := 0
			for i := range 200 {
				action := mix[i%len(mix)]
				if action == "Fail" {
					failures++
				}
				steps = append(steps, fmt.Sprintf("%v %s", float64(i+1)*3/60, action))
			}
			play(t, root, cwd, append(steps, "10.1 Stop")...)
			if log := errorLog(t, root); log != "" {
				t.Fatalf("hook-errors.log = %q", log)
			}

			list, data, set := recorded(t, root)
			if len(list) > tc.most {
				t.Fatalf("%d activity records for 200 tool results in ten minutes, want at most %d", len(list), tc.most)
			}
			tools, failed := 0, 0
			var paths []string
			for _, d := range data {
				tools, failed, paths = tools+d.Tools, failed+d.Failed, append(paths, d.Paths...)
			}
			if tools != 200 || failed != failures {
				t.Fatalf("records count %d tools and %d failures, want 200 and %d", tools, failed, failures)
			}
			if !slices.Equal(paths, []string{"src/a.ts", "src/b.ts"}) {
				t.Fatalf("recorded paths = %v, want each once", paths)
			}
			run := set.Get("fake:s1")
			if run.Tools != 200 || run.Edits != 2 || !set.Dirty("fake:s1") || set.State("fake:s1", minutes(10.2), runs.DefaultSettings()) != runs.Waiting {
				t.Fatalf("run: tools %d edits %d dirty %v", run.Tools, run.Edits, set.Dirty("fake:s1"))
			}
			// Every record sits within its interval of the run's previous
			// event, so last activity is never more than one interval late.
			if last := list[len(list)-1]; !last.Time.Equal(minutes(10.1)) {
				t.Fatalf("the turn end's record is at %v", last.Time)
			}
		})
	}
}

// An edited path is recorded once per run between checkpoints: the
// checkpoint tool's own result forgets the session's recorded paths, and a
// subagent records its own.
func TestEditPathsAreRecordedOncePerRunBetweenCheckpoints(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), repo(t)
	play(t, root, cwd, "0 SessionStart", "0 Prompt",
		"0.1 Edit:src/a.ts", "0.2 Edit:src/a.ts", "0.3 Edit:src/b.ts", "0.4 Edit:src/a.ts",
		"0.5 checkpoint", "0.5 Checkpointed",
		"0.6 Edit:src/a.ts", "0.65 Edit:src/b.ts",
		"0.7 SubagentStart", "0.8 Sub:Edit:src/a.ts", "0.9 Sub:Edit:src/a.ts", "1 Stop")
	list, data, set := recorded(t, root)
	var got []string
	for i, d := range data {
		for _, path := range d.Paths {
			got = append(got, list[i].Run+" "+path)
		}
	}
	want := []string{"fake:s1 src/a.ts", "fake:s1 src/b.ts", "fake:s1 src/a.ts", "fake:s1 src/b.ts", "fake:s1/a1 src/a.ts"}
	if !slices.Equal(got, want) {
		t.Fatalf("recorded paths =\n%v\nwant\n%v", got, want)
	}
	if set.Edits("fake:s1") != 3 || !set.Dirty("fake:s1") {
		t.Fatalf("edits since the checkpoint = %d, want 3 (two of the session's, one of its subagent's)", set.Edits("fake:s1"))
	}
}

// A permission request waits on the run's next tool result, so that result
// is recorded at once, however recent the run's last record.
func TestAPermissionRequestRecordsTheNextToolResult(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), repo(t)
	settings := runs.DefaultSettings()
	play(t, root, cwd, "0 SessionStart", "0 Prompt", "0.1 Read", "0.2 Permission", "0.3 Read")
	if _, _, set := recorded(t, root); set.State("fake:s1", minutes(0.35), settings) != runs.Working {
		t.Fatalf("state after the tool result = %s, want working", set.State("fake:s1", minutes(0.35), settings))
	}
	play(t, root, cwd, "0.4 SubagentStart", "0.5 Sub:Permission", "0.6 Sub:Read")
	list, data, set := recorded(t, root)
	if state := set.State("fake:s1", minutes(0.65), settings); state != runs.Working {
		t.Fatalf("session state after its subagent's tool result = %s, want working", state)
	}
	if len(list) != 2 || data[0].Tools != 2 || list[1].Run != "fake:s1/a1" {
		t.Fatalf("records = %+v %+v", list, data)
	}
}

// A subagent's pending activity is recorded before its end, and a
// session's (with its live subagents') before its turn end; the session's
// end removes its state.
func TestEndsRecordWhatIsPending(t *testing.T) {
	t.Parallel()

	// A real checkout: the subagent's command makes the stop check it.
	root, cwd := t.TempDir(), gitRepo(t)
	play(t, root, cwd, "0 SessionStart", "0 Prompt", "0.1 SubagentStart", "0.2 Sub:Read", "0.3 Sub:Bash", "0.4 SubagentStop")
	list := readEvents(t, root, "demo")
	end := list[len(list)-1]
	if end.Kind != events.RunEnd || end.Run != "fake:s1/a1" {
		t.Fatalf("last event = %+v", end)
	}
	var d events.ActivityData
	_ = list[len(list)-2].Decode(&d)
	if list[len(list)-2].Kind != events.Activity || d.Tools != 2 || len(d.Shell) != 1 || !d.Shell[0].From.Equal(minutes(0.2)) || !d.Shell[0].To.Equal(minutes(0.3)) {
		t.Fatalf("subagent's record = %+v %+v", list[len(list)-2], d)
	}
	if state, err := os.ReadFile(stateFile(root)); err != nil || strings.Contains(string(state), "a1") {
		t.Fatalf("state after the subagent ended = %s, %v", state, err)
	}

	play(t, root, cwd, "0.5 SubagentStart", "0.6 Sub:Read", "0.7 Read", "0.8 Stop")
	list = readEvents(t, root, "demo")
	var kinds []string
	for _, e := range list[len(list)-3:] {
		kinds = append(kinds, e.Run+" "+e.Kind)
	}
	if want := []string{"fake:s1 activity", "fake:s1/a1 activity", "fake:s1 turn.end"}; !slices.Equal(kinds, want) {
		t.Fatalf("turn end events = %v, want %v", kinds, want)
	}
	play(t, root, cwd, "1 End")
	if _, err := os.Stat(stateFile(root)); !os.IsNotExist(err) {
		t.Fatalf("state after the session ended: %v", err)
	}
	if log := errorLog(t, root); log != "" {
		t.Fatalf("hook-errors.log = %q", log)
	}
}

// The state is Flashheart's own cache: an unreadable one starts afresh
// rather than failing the hook (HOOK-1).
func TestAMalformedActivityStateStartsAfresh(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), repo(t)
	play(t, root, cwd, "0 SessionStart")
	if err := os.MkdirAll(filepath.Dir(stateFile(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stateFile(root), []byte(`{"runs":{"fake:s1":{"tools":"many"`), 0o644); err != nil {
		t.Fatal(err)
	}
	play(t, root, cwd, "0.1 Read")
	if list, _, _ := recorded(t, root); len(list) != 1 {
		t.Fatalf("records = %+v", list)
	}
	if log := errorLog(t, root); log != "" {
		t.Fatalf("hook-errors.log = %q", log)
	}
}

// Tool results arriving together (parallel tool calls, subagents) all
// count: the state is read and written under the project lock.
func TestConcurrentToolResultsAllCount(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), repo(t)
	play(t, root, cwd, "0 SessionStart", "0 Prompt")
	var wg sync.WaitGroup
	for range 40 {
		wg.Go(func() { play(t, root, cwd, "0.5 Read") })
	}
	wg.Wait()
	play(t, root, cwd, "0.9 Stop")
	_, data, _ := recorded(t, root)
	tools := 0
	for _, d := range data {
		tools += d.Tools
	}
	if tools != 40 {
		t.Fatalf("records count %d tools, want 40", tools)
	}
}
