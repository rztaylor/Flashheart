package hooks

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
// the turn-end and session-end hooks check the worktree for files changed
// since the last checkpoint and record only whether there were any.
func TestEndsRecordWorktreeChangesAfterShellCommands(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		steps   []string
		write   bool
		changed bool
	}{
		{"a shell command changed a file, then the session ended", []string{"Bash", "write", "End"}, true, true},
		{"a read-only shell command, then the session ended", []string{"Bash", "End"}, false, false},
		{"a file changed with no shell command (the user's own edit)", []string{"write", "End"}, true, false},
		{"a shell change before the checkpoint", []string{"Bash", "write", "checkpoint", "End"}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, cwd := t.TempDir(), gitRepo(t)
			run(t, root, "SessionStart", "s1 "+cwd, fake{})
			for _, step := range tc.steps {
				switch step {
				case "write":
					// Written now, after the hooks' clock.
					if err := os.WriteFile(filepath.Join(cwd, "a.txt"), []byte("changed\n"), 0o644); err != nil {
						t.Fatal(err)
					}
				case "checkpoint":
					// A checkpoint after the write: it ages the write to before it.
					if err := os.Chtimes(filepath.Join(cwd, "a.txt"), now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
						t.Fatal(err)
					}
					s, _ := store.Open(root)
					_ = events.New(s).Append(events.Event{Time: now, Run: "fake:s1", Agent: "fake", Kind: events.Checkpoint, Project: "demo", Data: events.CheckpointData{Ticket: "DM-1"}})
					s.Close()
				default:
					run(t, root, step, "s1 "+cwd, fake{})
				}
			}
			_, ends := changedFlags(t, root)
			if len(ends) != 1 || ends[0] != tc.changed {
				t.Fatalf("run.end worktree_changed = %v, want [%v]", ends, tc.changed)
			}
			if log := errorLog(t, root); log != "" {
				t.Fatalf("hook-errors.log = %q", log)
			}
			data, _ := json.Marshal(readEvents(t, root, "demo"))
			if strings.Contains(string(data), "a.txt") {
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
	run(t, root, "SessionStart", "s1 "+cwd, fake{})
	run(t, root, "Bash", "s1 "+cwd, fake{})
	if err := os.WriteFile(filepath.Join(cwd, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout strings.Builder
	Run(Options{Root: root, Event: "End", Stdin: strings.NewReader("s1 " + cwd), Stdout: &stdout, Now: func() time.Time { return now }, Adapter: fake{}, ChangeTimeout: time.Nanosecond})
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
			run(t, root, "SessionStart", "s1 "+cwd, fake{})
			project := "name: demo\nkey: DM\nnext_id: 2\nrepos:\n  - " + cwd + "\nsettings:\n  enforce_handoff: true\n"
			if err := os.WriteFile(filepath.Join(root, "demo", "project.yaml"), []byte(project), 0o644); err != nil {
				t.Fatal(err)
			}
			writeTicket(t, root, "demo", "DM-1-card-panel", "---\nid: DM-1\nstatus: in-progress\ntype: feature\npriority: high\ncreated: 2026-10-01\nbranch: feature/demo\n---\n# Card panel\n")
			run(t, root, "Prompt", "s1 "+cwd, fake{})
			run(t, root, "Bash", "s1 "+cwd, fake{})
			if write {
				if err := os.WriteFile(filepath.Join(cwd, "b.txt"), []byte("new\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			out := run(t, root, "Stop", "s1 "+cwd, fake{})
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
