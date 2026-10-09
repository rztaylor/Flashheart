package hooks

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// gitRepo makes a real git checkout named demo on branch feature/demo with
// one commit, every file older than the hooks' clock.
func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "src", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := now.Add(-time.Hour)
	date := old.Format("2006-01-02T15:04:05Z")
	for _, args := range [][]string{{"init", "-q", "-b", "feature/demo"}, {"add", "."}, {"commit", "-q", "-m", "init"}} {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_COMMITTER_DATE="+date, "GIT_AUTHOR_DATE="+date)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	for _, name := range []string{"a.txt", ".", filepath.Join(".git", "logs", "HEAD")} {
		if err := os.Chtimes(filepath.Join(dir, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// minutes is the hooks' clock m minutes after now.
func minutes(m float64) time.Time { return now.Add(time.Duration(m * float64(time.Minute))) }

// play runs a session's script in a checkout: each step is "<minute>
// <action>", where the action is a fake adapter event, "touch <file>" (the
// file is written and dated at that minute, as a command or the user would
// leave it) or "checkpoint" (on DM-1). It returns the last hook's output.
func play(t *testing.T, root, cwd string, steps ...string) string {
	t.Helper()
	out := ""
	for _, step := range steps {
		fields := strings.Fields(step)
		minute, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			t.Fatalf("step %q: %v", step, err)
		}
		at := minutes(minute)
		switch fields[1] {
		case "touch":
			name := filepath.Join(cwd, fields[2])
			if err := os.WriteFile(name, []byte(step+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(name, at, at); err != nil {
				t.Fatal(err)
			}
		case "checkpoint":
			s, err := store.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			err = events.New(s).Append(events.Event{Time: at, Run: "fake:s1", Agent: "fake", Kind: events.Checkpoint, Project: "demo", Data: events.CheckpointData{Ticket: "DM-1"}})
			s.Close()
			if err != nil {
				t.Fatal(err)
			}
		default:
			var stdout strings.Builder
			Run(Options{Root: root, Event: fields[1], Stdin: strings.NewReader("s1 " + cwd), Stdout: &stdout, Now: func() time.Time { return at }, Adapter: fake{}, ChangeTimeout: 10 * time.Second})
			out = stdout.String()
		}
	}
	return out
}

func changedFlags(t *testing.T, root string) (turnEnds, runEnds []bool) {
	t.Helper()
	for _, e := range readEvents(t, root, "demo") {
		switch e.Kind {
		case events.TurnEnd:
			var data events.TurnEndData
			_ = e.Decode(&data)
			turnEnds = append(turnEnds, data.WorktreeChanged)
		case events.RunEnd:
			var data events.RunEndData
			_ = e.Decode(&data)
			runEnds = append(runEnds, data.WorktreeChanged)
		}
	}
	return turnEnds, runEnds
}

// FH-53: agents edit mostly through shell commands, which name no path, so
// the turn-end and session-end hooks check the worktree and record only
// whether a file changed during one of the session's shell commands since
// its last checkpoint. A command's window runs from the run's previous
// event to the command's tool.used, so files the user changes while the
// session waits are not the session's.
func TestEndsRecordWorktreeChangesMadeDuringShellCommands(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		steps   []string
		changed bool
	}{
		{"a file changed during a shell command, then the session ended",
			[]string{"0 SessionStart", "1 Prompt", "2 touch a.txt", "3 Bash", "10 End"}, true},
		{"a read-only shell command, then the session ended",
			[]string{"0 SessionStart", "1 Prompt", "3 Bash", "4 Stop", "10 End"}, false},
		{"a file changed with no shell command (the user's own edit)",
			[]string{"0 SessionStart", "1 Prompt", "2 touch a.txt", "4 Stop", "10 End"}, false},
		{"a file changed only while the session waited for the user",
			[]string{"0 SessionStart", "1 Prompt", "3 Bash", "4 Stop", "6 touch a.txt", "8 Prompt", "9 Stop", "10 End"}, false},
		{"a file created only after the session stopped",
			[]string{"0 SessionStart", "1 Prompt", "3 Bash", "4 Stop", "6 touch new.txt", "10 End"}, false},
		{"files changed both during a command and while waiting",
			[]string{"0 SessionStart", "1 Prompt", "3 Bash", "4 Stop", "2 touch a.txt", "6 touch b.txt", "10 End"}, true},
		{"a shell change before the checkpoint",
			[]string{"0 SessionStart", "1 Prompt", "2 touch a.txt", "3 Bash", "3.5 checkpoint", "4 Stop", "10 End"}, false},
		{"a change within the slack after the command's result",
			[]string{"0 SessionStart", "1 Prompt", "3 Bash", "3.02 touch a.txt", "10 End"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, cwd := t.TempDir(), gitRepo(t)
			play(t, root, cwd, tc.steps...)
			turns, ends := changedFlags(t, root)
			if len(ends) != 1 {
				t.Fatalf("run ends = %v", ends)
			}
			changed := slices.Contains(append(turns, ends...), true)
			if changed != tc.changed {
				t.Fatalf("worktree_changed on turn ends %v and the end %v, want %v", turns, ends, tc.changed)
			}
			if log := errorLog(t, root); log != "" {
				t.Fatalf("hook-errors.log = %q", log)
			}
			data, _ := json.Marshal(readEvents(t, root, "demo"))
			if strings.Contains(string(data), ".txt") {
				t.Fatalf("the log names a changed file:\n%s", data)
			}
		})
	}
}

// A worktree check that misses its deadline fails open: the end is still
// recorded, unflagged, and the failure is logged (HOOK-1).
func TestASlowWorktreeCheckFailsOpen(t *testing.T) {
	t.Parallel()

	root, cwd := t.TempDir(), gitRepo(t)
	play(t, root, cwd, "0 SessionStart", "2 touch a.txt", "3 Bash")
	var stdout strings.Builder
	Run(Options{Root: root, Event: "End", Stdin: strings.NewReader("s1 " + cwd), Stdout: &stdout, Now: func() time.Time { return minutes(10) }, Adapter: fake{}, ChangeTimeout: time.Nanosecond})
	if _, ends := changedFlags(t, root); len(ends) != 1 || ends[0] {
		t.Fatalf("run.end worktree_changed = %v, want [false]", ends)
	}
	if log := errorLog(t, root); !strings.Contains(log, "check the worktree for changes") {
		t.Fatalf("hook-errors.log = %q", log)
	}
	if stdout.Len() > 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

// A linked run whose shell commands changed files is blocked at its stop
// like one that used the edit tools (HOOK-6), and the turn end records the
// change so the run stays due until its next checkpoint.
func TestStopEnforcesHandoffAfterShellChanges(t *testing.T) {
	t.Parallel()

	for _, write := range []bool{true, false} {
		t.Run(map[bool]string{true: "changed", false: "read only"}[write], func(t *testing.T) {
			t.Parallel()
			root, cwd := t.TempDir(), gitRepo(t)
			play(t, root, cwd, "0 SessionStart")
			project := "name: demo\nkey: DM\nnext_id: 2\nrepos:\n  - " + cwd + "\nsettings:\n  enforce_handoff: true\n"
			if err := os.WriteFile(filepath.Join(root, "demo", "project.yaml"), []byte(project), 0o644); err != nil {
				t.Fatal(err)
			}
			writeTicket(t, root, "demo", "DM-1-card-panel", "---\nid: DM-1\nstatus: in-progress\ntype: feature\npriority: high\ncreated: 2026-10-01\nbranch: feature/demo\n---\n# Card panel\n")
			steps := []string{"1 Prompt", "3 Bash", "4 Stop"}
			if write {
				steps = []string{"1 Prompt", "2 touch b.txt", "3 Bash", "4 Stop"}
			}
			out := play(t, root, cwd, steps...)
			if blocked := strings.HasPrefix(out, "block:"); blocked != write {
				t.Fatalf("stop output = %q, blocked want %v", out, write)
			}
			turns, _ := changedFlags(t, root)
			if len(turns) != 1 || turns[0] != write {
				t.Fatalf("turn.end worktree_changed = %v, want [%v]", turns, write)
			}
		})
	}
}
