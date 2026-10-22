package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/protocol"
)

// The session-start hook refreshes the marked section in Codex's global
// AGENTS.md (agent-protocol §5.4, D53): only a section that exists, only
// that section, and never a file whose markers it cannot pair. The skill
// is installed and current, so only the section can change.
func TestCodexRefreshProtocolKeepsTheMarkedSectionCurrent(t *testing.T) {
	t.Parallel()

	stale := fixtureAgents(t, "stale")
	start, end, found, err := findSection(stale)
	if err != nil || !found {
		t.Fatalf("the stale fixture has no section: %v", err)
	}
	before, old, after := string(stale[:start]), string(stale[start:end]), string(stale[end:])
	current := before + section("codex") + after

	for _, test := range []struct {
		name    string
		agents  string // "" with missing set: no AGENTS.md
		missing bool
		want    string // the file afterwards; "" means unchanged
	}{
		{name: "out of date", agents: string(stale), want: current},
		{name: "current", agents: current},
		{name: "current, no newline at the end", agents: before + strings.TrimSuffix(section("codex"), "\n")},
		{name: "older begin comment", agents: before + "<!-- BEGIN FLASHHEART: an older comment -->\nOld.\n" + sectionEnd + "\n" + after, want: current},
		{name: "missing section", agents: string(fixtureAgents(t, "user"))},
		{name: "empty file", agents: ""},
		{name: "no AGENTS.md", missing: true},
		{name: "begin marker without end", agents: before + strings.Replace(old, sectionEnd, "", 1) + after},
		{name: "end marker without begin", agents: before + strings.Replace(old, sectionBegin("codex"), "", 1) + after},
		{name: "two sections", agents: before + old + "\n" + old + after},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			m := newCodexMachine(t, "")
			writeSkillFolder(t, m.options.codexSkillDir())
			if !test.missing {
				m.write("AGENTS.md", test.agents)
			}
			old := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			if !test.missing {
				if err := os.Chtimes(m.path("AGENTS.md"), old, old); err != nil {
					t.Fatal(err)
				}
			}

			note, err := Codex{}.RefreshProtocol(m.options)
			if err != nil {
				t.Fatalf("refresh: %v", err)
			}
			if test.missing {
				if note != "" || m.exists("AGENTS.md") {
					t.Fatalf("note %q; a missing AGENTS.md was created: %v", note, m.exists("AGENTS.md"))
				}
				return
			}
			got := m.read("AGENTS.md")
			if test.want == "" {
				info, err := os.Stat(m.path("AGENTS.md"))
				if note != "" || got != test.agents || err != nil || !info.ModTime().Equal(old) {
					t.Fatalf("note %q, file rewritten (%v):\n%s", note, err, got)
				}
				return
			}
			if got != test.want {
				t.Fatalf("AGENTS.md =\n%s\nwant\n%s", got, test.want)
			}
			if note != protocol.SectionUpdatedNote(m.path("AGENTS.md")) {
				t.Fatalf("note = %q", note)
			}
			if entries, _ := os.ReadDir(filepath.Dir(m.path("AGENTS.md"))); len(entries) != 1 {
				t.Fatalf("the refresh left files behind: %v", entries)
			}
		})
	}
}

// The refresh works in $CODEX_HOME when it is set, writes through a
// symlinked AGENTS.md to its target (a dotfiles manager's link stays a
// link) and keeps the file's permissions.
func TestCodexRefreshProtocolFollowsCodexHomeAndLinks(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "")
	writeSkillFolder(t, m.options.codexSkillDir())
	m.options.CodexHome = filepath.Join(m.home, "codex-home")
	dotfiles := filepath.Join(m.home, "dotfiles", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(dotfiles), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dotfiles, fixtureAgents(t, "stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(m.options.CodexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dotfiles, m.path("AGENTS.md")); err != nil {
		t.Fatal(err)
	}

	note, err := Codex{}.RefreshProtocol(m.options)
	if err != nil || note == "" {
		t.Fatalf("note %q, err %v", note, err)
	}
	if info, err := os.Lstat(m.path("AGENTS.md")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was replaced: %v", err)
	}
	info, err := os.Stat(dotfiles)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, err %v", info.Mode(), err)
	}
	if got, _ := os.ReadFile(dotfiles); !strings.Contains(string(got), section("codex")) {
		t.Fatalf("the link's target was not refreshed:\n%s", got)
	}
	if m.exists(filepath.Join("..", ".codex", "AGENTS.md")) {
		t.Fatal("refresh wrote ~/.codex although CODEX_HOME is set")
	}
}

func TestCodexRefreshProtocolReportsAWriteFailure(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	m := newCodexMachine(t, "")
	writeSkillFolder(t, m.options.codexSkillDir())
	m.write("AGENTS.md", string(fixtureAgents(t, "stale")))
	dir := filepath.Dir(m.path("AGENTS.md"))
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if note, err := (Codex{}).RefreshProtocol(m.options); err == nil || note != "" {
		t.Fatalf("note %q, err %v", note, err)
	}
	if got := m.read("AGENTS.md"); got != string(fixtureAgents(t, "stale")) {
		t.Fatalf("AGENTS.md changed after a failed write:\n%s", got)
	}
}

// An install from before the skill folder has the whole protocol in its
// AGENTS.md section and no skill: the refresh writes the skill first, then
// shrinks the section to its pointer, so the protocol is never lost. A
// home without the section is left without both.
func TestCodexRefreshProtocolInstallsTheSkillBesideItsSection(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "")
	m.write("AGENTS.md", string(fixtureAgents(t, "stale")))
	note, err := Codex{}.RefreshProtocol(m.options)
	if err != nil {
		t.Fatal(err)
	}
	if want := protocol.SkillUpdatedNote + "\n" + protocol.SectionUpdatedNote(m.path("AGENTS.md")); note != want {
		t.Fatalf("note = %q, want %q", note, want)
	}
	checkSkillFolder(t, m.options.codexSkillDir())
	if got := m.read("AGENTS.md"); !strings.Contains(got, section("codex")) || strings.Contains(got, "An older version") {
		t.Fatalf("AGENTS.md:\n%s", got)
	}

	bare := newCodexMachine(t, "")
	bare.write("AGENTS.md", string(fixtureAgents(t, "user")))
	if note, err := (Codex{}).RefreshProtocol(bare.options); err != nil || note != "" {
		t.Fatalf("note %q, err %v", note, err)
	}
	if _, err := os.Stat(filepath.Join(bare.home, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("the refresh installed a skill for a home that was never set up: %v", err)
	}
}

// A Flashheart skill copied by hand into ~/.agents/skills is Flashheart's,
// and kept current like one setup wrote; a skill of the user's by that
// name is left alone, section or not.
func TestCodexRefreshProtocolKeepsOnlyFlashheartsSkill(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "")
	skill := filepath.Join(m.options.codexSkillDir(), "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte(staleSkill), 0o644); err != nil {
		t.Fatal(err)
	}
	if note, err := (Codex{}).RefreshProtocol(m.options); err != nil || note != protocol.SkillUpdatedNote {
		t.Fatalf("note %q, err %v", note, err)
	}
	checkSkillFolder(t, m.options.codexSkillDir())

	theirs := "---\nname: flashheart\ndescription: mine\n---\n\nMine.\n"
	if err := os.WriteFile(skill, []byte(theirs), 0o644); err != nil {
		t.Fatal(err)
	}
	m.write("AGENTS.md", section("codex"))
	if note, err := (Codex{}).RefreshProtocol(m.options); err != nil || note != "" {
		t.Fatalf("note %q, err %v", note, err)
	}
	if got, _ := os.ReadFile(skill); string(got) != theirs {
		t.Fatalf("the user's skill changed:\n%s", got)
	}
}

// Claude Code's session start also keeps the marked section of the global
// CLAUDE.md current, through a dotfiles link, and never adds one.
func TestClaudeRefreshProtocolKeepsTheCLAUDEmdSectionCurrent(t *testing.T) {
	t.Parallel()

	home, _ := installedSkill(t, "")
	writeSkillFolder(t, Options{Home: home}.claudeSkillDir())
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")
	dotfiles := filepath.Join(home, "dotfiles", "CLAUDE.md")
	mine := "# Mine\n\n- Be brief.\n\n"
	if err := os.MkdirAll(filepath.Dir(dotfiles), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dotfiles, []byte(mine+sectionBegin("claude")+"\nOld pointer.\n"+sectionEnd+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dotfiles, claudeMD); err != nil {
		t.Fatal(err)
	}
	note, err := (Claude{}).RefreshProtocol(Options{Home: home})
	if err != nil || note != protocol.SectionUpdatedNote(claudeMD) {
		t.Fatalf("note %q, err %v", note, err)
	}
	if info, err := os.Lstat(claudeMD); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was replaced: %v", err)
	}
	if got, _ := os.ReadFile(dotfiles); string(got) != mine+section("claude") {
		t.Fatalf("CLAUDE.md:\n%s", got)
	}

	for name, content := range map[string]string{"without a section": mine, "missing": ""} {
		home, _ := installedSkill(t, "")
		writeSkillFolder(t, Options{Home: home}.claudeSkillDir())
		claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")
		if content != "" {
			if err := os.WriteFile(claudeMD, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if note, err := (Claude{}).RefreshProtocol(Options{Home: home}); err != nil || note != "" {
			t.Fatalf("%s: note %q, err %v", name, note, err)
		}
		if got, err := os.ReadFile(claudeMD); string(got) != content || (content == "") != os.IsNotExist(err) {
			t.Fatalf("%s: CLAUDE.md = %q, %v", name, got, err)
		}
	}
}

// Claude Code's refresher is the skill's (skill_test.go), seen through the
// same interface the hook calls.
func TestClaudeRefreshProtocolNotesASkillUpdate(t *testing.T) {
	t.Parallel()

	home, _ := installedSkill(t, staleSkill)
	if note, err := (Claude{}).RefreshProtocol(Options{Home: home}); err != nil || note != protocol.SkillUpdatedNote {
		t.Fatalf("note %q, err %v", note, err)
	}
	if note, err := (Claude{}).RefreshProtocol(Options{Home: home}); err != nil || note != "" {
		t.Fatalf("second refresh: note %q, err %v", note, err)
	}
}

func fixtureAgents(t *testing.T, fixture string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "setup", "codex", fixture, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
