package setup

import (
	"os"
	"path/filepath"
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

func TestRefreshSkillRewritesAStaleFlashheartSkill(t *testing.T) {
	t.Parallel()

	home, path := installedSkill(t, staleSkill)
	updated, err := RefreshSkill(home)
	if err != nil || !updated {
		t.Fatalf("updated = %v, err = %v", updated, err)
	}
	// Edits are overwritten, as setup overwrites them: the skill is Flashheart's.
	if got, _ := os.ReadFile(path); string(got) != protocol.Skill() {
		t.Fatalf("skill not rewritten:\n%s", got)
	}
}

func TestRefreshSkillLeavesACurrentSkillAlone(t *testing.T) {
	t.Parallel()

	home, path := installedSkill(t, protocol.Skill())
	old := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	updated, err := RefreshSkill(home)
	if err != nil || updated {
		t.Fatalf("updated = %v, err = %v", updated, err)
	}
	if info, err := os.Stat(path); err != nil || !info.ModTime().Equal(old) {
		t.Fatalf("a current skill was written: %v %v", info.ModTime(), err)
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
			updated, err := RefreshSkill(home)
			if err != nil || updated {
				t.Fatalf("updated = %v, err = %v", updated, err)
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
	if updated, err := RefreshSkill(home); err == nil || updated {
		t.Fatalf("updated = %v, err = %v", updated, err)
	}
}
