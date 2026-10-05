package gitinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// layout builds a repository by hand: a main checkout with a linked
// worktree, a detached worktree and a submodule.
func layout(t *testing.T) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(base, "src", "ngplus")
	write(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/main\n")
	write(t, filepath.Join(repo, "web", "src", "app.ts"), "")
	// Linked worktree on a feature branch.
	wt := filepath.Join(base, "wt", "feature-x")
	write(t, filepath.Join(wt, ".git"), "gitdir: "+filepath.Join(repo, ".git", "worktrees", "feature-x")+"\n")
	write(t, filepath.Join(repo, ".git", "worktrees", "feature-x", "HEAD"), "ref: refs/heads/feature/x\n")
	write(t, filepath.Join(repo, ".git", "worktrees", "feature-x", "commondir"), "../..\n")
	// Detached worktree with a relative gitdir.
	detached := filepath.Join(repo, "detached")
	write(t, filepath.Join(detached, ".git"), "gitdir: ../.git/worktrees/detached\n")
	write(t, filepath.Join(repo, ".git", "worktrees", "detached", "HEAD"), "3f2a9c1e0b7d4a5f9e8c7b6a5d4c3b2a1f0e9d8c\n")
	write(t, filepath.Join(repo, ".git", "worktrees", "detached", "commondir"), "../..\n")
	// Submodule: its git directory lives under the parent's .git/modules.
	sub := filepath.Join(repo, "vendor", "lib")
	write(t, filepath.Join(sub, ".git"), "gitdir: ../../.git/modules/lib\n")
	write(t, filepath.Join(repo, ".git", "modules", "lib", "HEAD"), "ref: refs/heads/trunk\n")
	return base
}

func TestResolve(t *testing.T) {
	t.Parallel()

	base := layout(t)
	repo := filepath.Join(base, "src", "ngplus")
	cases := []struct {
		name, cwd                       string
		project, repo, worktree, branch string
	}{
		{"main checkout", repo, "ngplus", repo, repo, "main"},
		{"subdirectory", filepath.Join(repo, "web", "src"), "ngplus", repo, repo, "main"},
		{"linked worktree maps to the main checkout", filepath.Join(base, "wt", "feature-x"), "ngplus", repo, filepath.Join(base, "wt", "feature-x"), "feature/x"},
		{"detached head records the short sha", filepath.Join(repo, "detached"), "ngplus", repo, filepath.Join(repo, "detached"), "3f2a9c1"},
		{"submodule is its own project", filepath.Join(repo, "vendor", "lib"), "lib", filepath.Join(repo, "vendor", "lib"), filepath.Join(repo, "vendor", "lib"), "trunk"},
		{"outside git", base, "", "", "", ""},
		{"missing directory", filepath.Join(base, "nope"), "", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Resolve(tc.cwd)
			if got.Project != tc.project || got.Repo != tc.repo || got.Worktree != tc.worktree || got.Branch != tc.branch {
				t.Fatalf("Resolve(%s) = %+v\nwant project=%s repo=%s worktree=%s branch=%s", tc.cwd, got, tc.project, tc.repo, tc.worktree, tc.branch)
			}
		})
	}
}

func TestResolveRealGitWorktree(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	base, _ := filepath.EvalSymlinks(t.TempDir())
	repo := filepath.Join(base, "flashheart")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run(repo, "init", "-q", "-b", "main")
	run(repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init")
	run(repo, "worktree", "add", "-q", "-b", "feature/agent-runs", filepath.Join(base, "wt"))
	got := Resolve(filepath.Join(base, "wt"))
	if got.Project != "flashheart" || got.Repo != repo || got.Branch != "feature/agent-runs" || got.Worktree != filepath.Join(base, "wt") {
		t.Fatalf("Resolve(worktree) = %+v", got)
	}
}

func TestCacheReusesUntilHeadChanges(t *testing.T) {
	t.Parallel()

	base := layout(t)
	repo := filepath.Join(base, "src", "ngplus")
	cache := ParseCache(nil)
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

	first, hit := cache.Resolve(repo, now)
	if hit || first.Branch != "main" {
		t.Fatalf("first Resolve = %+v, hit %v", first, hit)
	}
	// Round-trip through the file format.
	cache = ParseCache(cache.Marshal())
	if again, hit := cache.Resolve(repo, now); !hit || again != first {
		t.Fatalf("cached Resolve = %+v, hit %v", again, hit)
	}

	// Switching branches rewrites HEAD; its new mtime invalidates the entry.
	head := filepath.Join(repo, ".git", "HEAD")
	write(t, head, "ref: refs/heads/feature/y\n")
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(head, later, later); err != nil {
		t.Fatal(err)
	}
	if switched, hit := cache.Resolve(repo, now); hit || switched.Branch != "feature/y" {
		t.Fatalf("after checkout Resolve = %+v, hit %v", switched, hit)
	}

	// Outside git is cached too, validated by the directory's existence.
	if got, _ := cache.Resolve(base, now); got.Project != "" {
		t.Fatalf("outside git = %+v", got)
	}
	if _, hit := cache.Resolve(base, now); !hit {
		t.Fatal("outside git was not cached")
	}
}

func TestCacheIsBoundedAndToleratesGarbage(t *testing.T) {
	t.Parallel()

	if c := ParseCache([]byte("not json")); len(c.Entries) != 0 {
		t.Fatalf("garbage cache = %+v", c)
	}
	base := t.TempDir()
	cache := ParseCache(nil)
	start := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	for i := range MaxEntries + 10 {
		dir := filepath.Join(base, "dir", time.Duration(i).String())
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		cache.Resolve(dir, start.Add(time.Duration(i)*time.Second))
	}
	parsed := ParseCache(cache.Marshal())
	if len(parsed.Entries) != MaxEntries {
		t.Fatalf("cache has %d entries, want %d", len(parsed.Entries), MaxEntries)
	}
	if _, ok := parsed.Entries[filepath.Join(base, "dir", time.Duration(0).String())]; ok {
		t.Fatal("least recently used entry was kept")
	}
}

func TestRelative(t *testing.T) {
	t.Parallel()

	info := Info{Worktree: "/src/ngplus"}
	cases := map[string]string{
		"/src/ngplus/web/app.ts": "web/app.ts",
		"web/app.ts":             "web/app.ts",
		"/src/ngplus":            "",
		"/src/other/app.ts":      "",
		"/src/ngplus/../x":       "",
		"":                       "",
	}
	for in, want := range cases {
		if got := info.Relative(in); got != want {
			t.Errorf("Relative(%q) = %q, want %q", in, got, want)
		}
	}
	if got := (Info{}).Relative("/x/y"); got != "" {
		t.Errorf("Relative outside git = %q", got)
	}
}
