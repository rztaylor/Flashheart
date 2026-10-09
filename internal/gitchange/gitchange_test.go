package gitchange

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// fixture is a git checkout whose files all date from an hour ago, with
// since half an hour ago: only what a test touches is newer. counts, when
// a case sets it, limits which modification times count.
type fixture struct {
	t      *testing.T
	dir    string
	since  time.Time
	old    time.Time
	counts func(time.Time) bool
}

// outsideNow counts only modifications from since until ten minutes ago, so
// what a test writes now is outside it.
func (f *fixture) outsideNow() {
	f.counts = func(modified time.Time) bool { return modified.Before(time.Now().Add(-10 * time.Minute)) }
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, dir: dir, since: time.Now().Add(-30 * time.Minute), old: time.Now().Add(-time.Hour)}
	f.git("init", "-q", "-b", "main")
	f.write("a.txt", "a\n")
	f.write("docs/b.txt", "b\n")
	f.write(".gitignore", "build/\n")
	f.git("add", ".")
	date := f.old.Format(time.RFC3339)
	f.run([]string{"GIT_COMMITTER_DATE=" + date, "GIT_AUTHOR_DATE=" + date}, "commit", "-q", "-m", "init")
	f.age("a.txt", "docs/b.txt", ".gitignore", "docs", ".")
	// The commit's reflog entry is old too.
	f.age(filepath.Join(".git", "logs", "HEAD"))
	return f
}

// git runs git in the checkout; commits are dated now.
func (f *fixture) git(args ...string) { f.t.Helper(); f.run(nil, args...) }

func (f *fixture) run(env []string, args ...string) {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
	cmd.Dir = f.dir
	cmd.Env = append(append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null"), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		f.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (f *fixture) write(name, content string) {
	f.t.Helper()
	path := filepath.Join(f.dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) age(names ...string) {
	f.t.Helper()
	for _, name := range names {
		if err := os.Chtimes(filepath.Join(f.dir, filepath.FromSlash(name)), f.old, f.old); err != nil {
			f.t.Fatal(err)
		}
	}
}

func (f *fixture) changed() bool {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	changed, err := ChangedSince(ctx, f.dir, filepath.Join(f.dir, ".git"), f.since, f.counts)
	if err != nil {
		f.t.Fatal(err)
	}
	return changed
}

func TestChangedSince(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		act    func(f *fixture)
		change bool
	}{
		{"untouched", func(*fixture) {}, false},
		{"tracked file edited", func(f *fixture) { f.write("a.txt", "a2\n") }, true},
		{"new untracked file", func(f *fixture) { f.write("docs/new/c.txt", "c\n") }, true},
		{"edited before since and left uncommitted", func(f *fixture) { f.write("a.txt", "a2\n"); f.age("a.txt") }, false},
		{"file deleted", func(f *fixture) {
			if err := os.Remove(filepath.Join(f.dir, "docs", "b.txt")); err != nil {
				f.t.Fatal(err)
			}
		}, true},
		{"directory removed", func(f *fixture) {
			if err := os.RemoveAll(filepath.Join(f.dir, "docs")); err != nil {
				f.t.Fatal(err)
			}
		}, true},
		{"edited and committed", func(f *fixture) {
			f.write("a.txt", "a2\n")
			f.git("commit", "-q", "-am", "edit")
		}, true},
		{"older edit committed later", func(f *fixture) {
			f.write("a.txt", "a2\n")
			f.age("a.txt")
			f.git("commit", "-q", "-am", "edit")
		}, false},
		{"ignored file written", func(f *fixture) { f.write("build/out.bin", "x") }, false},
		{"branch switched", func(f *fixture) { f.git("checkout", "-q", "-b", "other") }, false},
		{"edited outside the counted times", func(f *fixture) { f.outsideNow(); f.write("a.txt", "a2\n") }, false},
		{"edited and committed outside the counted times", func(f *fixture) {
			f.outsideNow()
			f.write("a.txt", "a2\n")
			f.git("commit", "-q", "-am", "edit")
		}, false},
		{"edited inside the counted times", func(f *fixture) {
			f.outsideNow()
			f.write("a.txt", "a2\n")
			inside := time.Now().Add(-20 * time.Minute)
			if err := os.Chtimes(filepath.Join(f.dir, "a.txt"), inside, inside); err != nil {
				f.t.Fatal(err)
			}
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			tc.act(f)
			if got := f.changed(); got != tc.change {
				t.Fatalf("changed = %v, want %v", got, tc.change)
			}
		})
	}
}

func TestChangedSinceOutsideGitFails(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	if _, err := ChangedSince(context.Background(), dir, filepath.Join(dir, ".git"), time.Now(), nil); err == nil {
		t.Fatal("want an error outside a repository")
	}
}

func TestChangedSinceGivesUpAtTheDeadline(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.write("a.txt", "a2\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ChangedSince(ctx, f.dir, filepath.Join(f.dir, ".git"), f.since, f.counts); err == nil {
		t.Fatal("want an error once the context is done")
	}
}
