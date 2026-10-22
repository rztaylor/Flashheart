package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/protocol"
)

func installedSkill(t *testing.T, content string) (home, path string) {
	t.Helper()
	home = t.TempDir()
	path = filepath.Join(home, ".claude", "skills", protocol.SkillName, "SKILL.md")
	if content == "" {
		return home, path
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return home, path
}

const staleSkill = "---\nname: flashheart\ndescription: old\nmetadata:\n  flashheart-protocol: 1\n---\n\n# Flashheart board\n\nOld text, hand-edited.\n"

// writeSkillFolder writes this binary's skill, every file, into dir.
func writeSkillFolder(t *testing.T, dir string) {
	t.Helper()
	for _, file := range protocol.SkillFiles() {
		path := filepath.Join(dir, file.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(file.Text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// checkSkillFolder fails unless dir holds this binary's skill, every file.
func checkSkillFolder(t *testing.T, dir string) {
	t.Helper()
	for _, file := range protocol.SkillFiles() {
		if got, err := os.ReadFile(filepath.Join(dir, file.Path)); err != nil || string(got) != file.Text {
			t.Errorf("%s is not this binary's (%v):\n%.200s", file.Path, err, got)
		}
	}
}

// An install from before the skill had pages has only SKILL.md: the
// refresh rewrites it and adds the pages beside it.
func TestRefreshSkillRewritesAStaleFlashheartSkill(t *testing.T) {
	t.Parallel()

	home, path := installedSkill(t, staleSkill)
	updated, err := Claude{}.RefreshSkill(home)
	if err != nil || !updated {
		t.Fatalf("updated = %v, err = %v", updated, err)
	}
	// Edits are overwritten, as setup overwrites them: the skill is Flashheart's.
	checkSkillFolder(t, filepath.Dir(path))
}

// One out-of-date or missing page is rewritten alone.
func TestRefreshSkillRewritesAnOutOfDatePage(t *testing.T) {
	t.Parallel()

	for name, change := range map[string]func(path string) error{
		"edited":  func(path string) error { return os.WriteFile(path, []byte("# Old page\n"), 0o644) },
		"missing": os.Remove,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			home, path := installedSkill(t, "")
			dir := filepath.Dir(path)
			writeSkillFolder(t, dir)
			old := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
			if err := change(filepath.Join(dir, "references", "finishing.md")); err != nil {
				t.Fatal(err)
			}
			updated, err := Claude{}.RefreshSkill(home)
			if err != nil || !updated {
				t.Fatalf("updated = %v, err = %v", updated, err)
			}
			checkSkillFolder(t, dir)
			if info, err := os.Stat(path); err != nil || !info.ModTime().Equal(old) {
				t.Fatalf("a current SKILL.md was written: %v", err)
			}
		})
	}
}

func TestRefreshSkillLeavesACurrentSkillAlone(t *testing.T) {
	t.Parallel()

	home, path := installedSkill(t, "")
	dir := filepath.Dir(path)
	writeSkillFolder(t, dir)
	old := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, file := range protocol.SkillFiles() {
		if err := os.Chtimes(filepath.Join(dir, file.Path), old, old); err != nil {
			t.Fatal(err)
		}
	}
	updated, err := Claude{}.RefreshSkill(home)
	if err != nil || updated {
		t.Fatalf("updated = %v, err = %v", updated, err)
	}
	for _, file := range protocol.SkillFiles() {
		if info, err := os.Stat(filepath.Join(dir, file.Path)); err != nil || !info.ModTime().Equal(old) {
			t.Fatalf("a current %s was written: %v", file.Path, err)
		}
	}
}

func TestRefreshSkillNeverInstalls(t *testing.T) {
	t.Parallel()

	for name, content := range map[string]string{
		"missing (never set up, or uninstalled)": "",
		"not Flashheart's":                       "---\nname: flashheart\ndescription: someone else's\n---\n\nTheirs.\n",
		"no frontmatter":                         "# Flashheart board\n\nflashheart-protocol: 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			home, path := installedSkill(t, content)
			updated, err := Claude{}.RefreshSkill(home)
			if err != nil || updated {
				t.Fatalf("updated = %v, err = %v", updated, err)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), "references")); !os.IsNotExist(err) {
				t.Fatalf("pages were written beside a skill that is not Flashheart's: %v", err)
			}
			got, err := os.ReadFile(path)
			if content == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("a missing skill was created: %v", err)
				}
				return
			}
			if string(got) != content {
				t.Fatalf("a skill that is not Flashheart's changed:\n%s", got)
			}
		})
	}
}

func TestRefreshSkillReportsAWriteFailure(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	home, path := installedSkill(t, staleSkill)
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if updated, err := (Claude{}).RefreshSkill(home); err == nil || updated {
		t.Fatalf("updated = %v, err = %v", updated, err)
	}
}

// ~/.claude/skills and ~/.agents/skills may link to one folder of the
// user's: uninstalling one agent leaves the skill the other still uses,
// saying so, and the last uninstall removes it.
func TestUninstallKeepsASkillFolderTheOtherAgentShares(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	shared := filepath.Join(m.home, "src", "skills")
	for _, link := range []string{filepath.Join(m.home, ".claude", "skills"), filepath.Join(m.home, ".agents", "skills")} {
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(shared, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(shared, link); err != nil {
			t.Fatal(err)
		}
	}
	codex := m.options
	codex.CLI = ""
	m.apply(true)
	applyCodex(t, codex, true)
	checkSkillFolder(t, filepath.Join(shared, "flashheart"))

	plan, err := Codex{}.Uninstall(codex)
	if err != nil {
		t.Fatal(err)
	}
	if out := render(plan); strings.Contains(out, "Delete ~/.agents/skills/flashheart") || !strings.Contains(out, "also Claude Code's") {
		t.Fatalf("codex uninstall plan:\n%s", out)
	}
	applyCodex(t, codex, false)
	checkSkillFolder(t, filepath.Join(shared, "flashheart"))

	m.apply(false)
	if _, err := os.Stat(filepath.Join(shared, "flashheart")); !os.IsNotExist(err) {
		t.Fatalf("the last uninstall left the skill: %v", err)
	}
}

func applyCodex(t *testing.T, o Options, install bool) {
	t.Helper()
	plan, err := Codex{}.Install(o)
	if !install {
		plan, err = Codex{}.Uninstall(o)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}
