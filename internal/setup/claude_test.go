package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/protocol"
)

var fixture = filepath.Join("..", "..", "testdata", "setup", "claude", "settings.json")

type machine struct {
	t        *testing.T
	home     string
	options  Options
	ran      [][]string
	original []byte
	// server is the flashheart MCP registration the fake claude CLI holds:
	// its JSON config, or "" when none.
	server string
	// queried counts claude mcp get calls.
	queried int
	// scope is the Scope line claude mcp get prints; empty means user.
	scope string
}

// newMachine is a home with the fixture settings, a kanban-tracker skill
// and a claude CLI that records what it is asked to do.
func newMachine(t *testing.T) *machine {
	t.Helper()
	home := t.TempDir()
	original, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	m := &machine{t: t, home: home, original: original}
	m.write(".claude/settings.json", string(original))
	m.write(".claude/skills/kanban-tracker/SKILL.md", "---\nname: kanban-tracker\n---\n# Kanban\n")
	clock := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	m.options = Options{
		Home: home, Binary: "/opt/Flash Heart/bin/flashheart", Claude: "/usr/local/bin/claude",
		Now: func() time.Time { clock = clock.Add(time.Second); return clock },
		Run: func(name string, args ...string) ([]byte, error) {
			// Mimic the claude CLI's user-scope MCP registry.
			switch args[1] {
			case "get":
				m.queried++
				return m.mcpGet()
			case "add-json":
				m.server = args[5]
			case "remove":
				m.server = ""
			}
			m.ran = append(m.ran, args)
			return nil, nil
		},
	}
	return m
}

// mcpGet answers claude mcp get flashheart as the CLI does.
func (m *machine) mcpGet() ([]byte, error) {
	if m.server == "" {
		return []byte(`No MCP server named "flashheart". Configured servers: other` + "\n"), errors.New("exit status 1")
	}
	var config struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal([]byte(m.server), &config); err != nil {
		m.t.Fatal(err)
	}
	scope := m.scope
	if scope == "" {
		scope = "User config (available in all your projects)"
	}
	return []byte("flashheart:\n  Scope: " + scope + "\n  Status: ✔ Connected\n  Type: stdio\n  Command: " + config.Command + "\n  Args: " + strings.Join(config.Args, " ") + "\n\nTo remove this server, run: claude mcp remove flashheart -s user\n"), nil
}

func (m *machine) write(name, data string) {
	m.t.Helper()
	path := filepath.Join(m.home, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		m.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		m.t.Fatal(err)
	}
}

func (m *machine) read(name string) []byte {
	m.t.Helper()
	data, err := os.ReadFile(filepath.Join(m.home, name))
	if err != nil {
		m.t.Fatal(err)
	}
	return data
}

func (m *machine) exists(name string) bool {
	_, err := os.Stat(filepath.Join(m.home, name))
	return err == nil
}

func (m *machine) plan(install bool) *Plan {
	m.t.Helper()
	var plan *Plan
	var err error
	if install {
		plan, err = Install(m.options)
	} else {
		plan, err = Uninstall(m.options)
	}
	if err != nil {
		m.t.Fatal(err)
	}
	return plan
}

func (m *machine) apply(install bool) string {
	m.t.Helper()
	var out bytes.Buffer
	if err := m.plan(install).Apply(&out); err != nil {
		m.t.Fatal(err)
	}
	return out.String()
}

func render(p *Plan) string {
	var out bytes.Buffer
	p.Render(&out)
	return out.String()
}

func TestInstallShowsADiffAndWritesNothing(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	out := render(m.plan(true))
	for _, part := range []string{
		"Change ~/.claude/settings.json:\n--- ~/.claude/settings.json\n+++ ~/.claude/settings.json\n@@ ",
		`+            "command": "'/opt/Flash Heart/bin/flashheart' hook claude SessionStart",`,
		`+        "matcher": "mcp__flashheart__.*",`,
		"Create ~/.claude/skills/flashheart/SKILL.md:",
		"+name: flashheart",
		"Move ~/.claude/skills/kanban-tracker to ~/.claude/flashheart-backup/20261006T120001Z-",
		`Run: claude mcp add-json --scope user flashheart '{"args":["mcp"],"command":"/opt/Flash Heart/bin/flashheart","type":"stdio"}'`,
	} {
		if !strings.Contains(out, part) {
			t.Fatalf("plan missing %q:\n%s", part, out)
		}
	}
	if !bytes.Equal(m.read(".claude/settings.json"), m.original) || m.exists(".claude/skills/flashheart") || !m.exists(".claude/skills/kanban-tracker") || len(m.ran) > 0 {
		t.Fatal("rendering the plan changed something")
	}
}

func TestInstallWritesKeepsTheUsersSettingsAndIsIdempotent(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	out := m.apply(true)
	for _, part := range []string{"Wrote ~/.claude/settings.json", "Wrote ~/.claude/skills/flashheart/SKILL.md", "Moved ~/.claude/skills/kanban-tracker", "Ran claude mcp add-json", "Backups are in ~/.claude/flashheart-backup/"} {
		if !strings.Contains(out, part) {
			t.Fatalf("output missing %q:\n%s", part, out)
		}
	}
	var settings struct {
		Model  string                       `json:"model"`
		Env    map[string]string            `json:"env"`
		Hooks  map[string][]json.RawMessage `json:"hooks"`
		Status map[string]string            `json:"statusLine"`
	}
	data := m.read(".claude/settings.json")
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Model != "opus" || settings.Env["EDITOR"] != "vim — “quoted” ünïcode" || settings.Status["command"] != "~/.claude/statusline.sh" {
		t.Fatalf("user settings changed: %+v", settings)
	}
	if len(settings.Hooks) != len(claudeEvents) || len(settings.Hooks["Notification"]) != 2 || len(settings.Hooks["PostToolUse"]) != 2 {
		t.Fatalf("hooks = %v", settings.Hooks)
	}
	// Values setup does not own keep their exact bytes, escapes included.
	for _, kept := range []string{`"NOTE": "escaped \u2014 dash, \"quote\" and tab\t kept as written"`, `"LIMIT": 1.50`} {
		if !strings.Contains(string(data), kept) {
			t.Fatalf("settings lost %s:\n%s", kept, data)
		}
	}
	if diff := unifiedDiff("s", m.original, data); strings.Contains(diff, "-    \"") {
		t.Fatalf("install changed lines outside the hooks:\n%s", diff)
	}
	if !strings.Contains(string(data), `"command": "afplay /System/Library/Sounds/Glass.aiff"`) || !strings.Contains(string(data), `"timeout": 5`) {
		t.Fatalf("settings:\n%s", data)
	}
	if string(m.read(".claude/skills/flashheart/SKILL.md")) != protocol.Skill() || m.exists(".claude/skills/kanban-tracker") {
		t.Fatal("skills not swapped")
	}
	if again := m.plan(true); !again.Empty() {
		t.Fatalf("second install is not empty:\n%s", render(again))
	}
}

func TestUninstallRestoresTheOriginalByteForByte(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.apply(true)
	out := render(m.plan(false))
	for _, part := range []string{"Change ~/.claude/settings.json:", "Delete ~/.claude/skills/flashheart/SKILL.md.", "Move ~/.claude/flashheart-backup/", "Run: claude mcp remove --scope user flashheart"} {
		if !strings.Contains(out, part) {
			t.Fatalf("uninstall plan missing %q:\n%s", part, out)
		}
	}
	m.apply(false)
	if got := m.read(".claude/settings.json"); !bytes.Equal(got, m.original) {
		t.Fatalf("settings after uninstall:\n%s", unifiedDiff("settings.json", m.original, got))
	}
	if m.exists(".claude/skills/flashheart") || !m.exists(".claude/skills/kanban-tracker/SKILL.md") {
		t.Fatal("skills not restored")
	}
	if last := m.ran[len(m.ran)-1]; !slices.Equal(last, []string{"mcp", "remove", "--scope", "user", "flashheart"}) {
		t.Fatalf("commands = %v", m.ran)
	}
	if again := m.plan(false); !again.Empty() {
		t.Fatalf("second uninstall is not empty:\n%s", render(again))
	}
}

func TestUninstallAfterTheUserChangedSettingsRemovesOnlyFlashheartHooks(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.apply(true)
	edited := strings.Replace(string(m.read(".claude/settings.json")), `"model": "opus"`, `"model": "sonnet"`, 1)
	m.write(".claude/settings.json", edited)
	m.apply(false)
	data := string(m.read(".claude/settings.json"))
	if strings.Contains(data, "flashheart") || !strings.Contains(data, `"model": "sonnet"`) || !strings.Contains(data, "afplay") || !strings.Contains(data, "prettier") {
		t.Fatalf("settings:\n%s", data)
	}
}

func TestInstallReplacesHandMadeFlashheartHooks(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.write(".claude/settings.json", `{"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "/Users/me/bin/flashheart hook claude Stop"}]}], "SessionStart": [{"hooks": [{"type": "command", "command": "echo hi"}, {"type": "command", "command": "flashheart hook claude SessionStart --root /b"}]}]}}`+"\n")
	m.options.Root = "/data/board"
	m.apply(true)
	data := string(m.read(".claude/settings.json"))
	if strings.Count(data, "hook claude Stop") != 1 || strings.Contains(data, "/Users/me/bin") || !strings.Contains(data, "echo hi") || !strings.Contains(data, "hook claude Stop --root /data/board") {
		t.Fatalf("settings:\n%s", data)
	}
	if !strings.Contains(m.server, `"args":["mcp","--root","/data/board"]`) {
		t.Fatalf("mcp registration: %s", m.server)
	}
	// Uninstall takes Flashheart's hooks away, including the hand-made ones.
	m.apply(false)
	data = string(m.read(".claude/settings.json"))
	if strings.Contains(data, "flashheart") || !strings.Contains(data, "echo hi") {
		t.Fatalf("settings after uninstall:\n%s", data)
	}
}

func TestInstallReregistersAMovedBinary(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.server = `{"type": "stdio", "command": "/old/flashheart", "args": ["mcp"]}`
	plan := m.plan(true)
	if len(plan.commands) != 2 || plan.commands[0][1] != "remove" || plan.commands[1][1] != "add-json" {
		t.Fatalf("commands = %v", plan.commands)
	}
}

// The registration is read from the claude CLI, never by parsing
// ~/.claude.json, which Claude Code owns (FH-8).
func TestTheRegistrationComesFromTheClaudeCLI(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.write(".claude.json", "{ not json")
	m.server = `{"type": "stdio", "command": "/opt/Flash Heart/bin/flashheart", "args": ["mcp"]}`
	plan := m.plan(true)
	if m.queried == 0 || len(plan.commands) != 0 {
		t.Fatalf("queried %d, commands %v", m.queried, plan.commands)
	}
	if strings.Contains(render(plan), ".claude.json") {
		t.Errorf("plan mentions ~/.claude.json:\n%s", render(plan))
	}
}

// Without the CLI, an unreadable ~/.claude.json is reported, not taken as
// "not registered".
func TestAnUnreadableClaudeJSONIsReported(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.options.Claude = ""
	m.write(".claude.json", "{ not json")
	if out := render(m.plan(true)); !strings.Contains(out, "Could not read ~/.claude.json") {
		t.Fatalf("plan:\n%s", out)
	}
	m.write(".claude.json", `{"mcpServers": {"flashheart": {"type": "stdio", "command": "/opt/Flash Heart/bin/flashheart", "args": ["mcp"]}}}`)
	if plan := m.plan(true); len(plan.commands) != 0 {
		t.Fatalf("commands = %v", plan.commands)
	}
}

func TestWithoutTheClaudeCLITheCommandsArePrinted(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.options.Claude = ""
	out := m.apply(true)
	if !strings.Contains(out, "The claude command was not found, so register the MCP server yourself:\n  claude mcp add-json --scope user flashheart") || len(m.ran) > 0 {
		t.Fatalf("output:\n%s", out)
	}
}

func TestAFailedRegistrationNamesWhatIsLeft(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.options.Run = func(string, ...string) ([]byte, error) { return []byte("boom"), errors.New("exit status 1") }
	err := m.plan(true).Apply(&bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "Run the rest yourself:\n  claude mcp add-json") {
		t.Fatalf("err = %v", err)
	}
}

func TestBrokenSettingsAreRefused(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.write(".claude/settings.json", "{ not json")
	if _, err := Install(m.options); err == nil || !strings.Contains(err.Error(), "settings.json is not a JSON object") {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallOnAFreshMachineAndBack(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	if err := os.Remove(filepath.Join(m.home, ".claude", "settings.json")); err != nil {
		t.Fatal(err)
	}
	m.apply(true)
	if !strings.Contains(string(m.read(".claude/settings.json")), "hook claude SessionEnd") {
		t.Fatal("hooks not written")
	}
	m.apply(false)
	if m.exists(".claude/settings.json") {
		t.Fatalf("settings.json was absent before setup but remains:\n%s", m.read(".claude/settings.json"))
	}
}

func TestASymlinkedSettingsFileStaysALink(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	dotfiles := filepath.Join(m.home, "dotfiles", "claude-settings.json")
	m.write("dotfiles/claude-settings.json", string(m.original))
	settings := filepath.Join(m.home, ".claude", "settings.json")
	if err := os.Remove(settings); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dotfiles, settings); err != nil {
		t.Fatal(err)
	}
	m.apply(true)
	if info, err := os.Lstat(settings); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("settings.json is no longer a link: %v", err)
	}
	if data, _ := os.ReadFile(dotfiles); !strings.Contains(string(data), "hook claude SessionStart") {
		t.Fatal("the link's target was not updated")
	}
	m.apply(false)
	if data, _ := os.ReadFile(dotfiles); !bytes.Equal(data, m.original) {
		t.Fatal("uninstall did not restore the link's target")
	}
}

func TestHooksSetupCannotReadAreRefused(t *testing.T) {
	t.Parallel()

	for name, settings := range map[string]string{
		"hooks not an object": `{"hooks": []}`,
		"event not a list":    `{"hooks": {"Stop": {"oops": 1}}}`,
		"group not an object": `{"hooks": {"Stop": ["echo"]}}`,
		"entries not a list":  `{"hooks": {"Stop": [{"hooks": {"type": "command"}}]}}`,
	} {
		m := newMachine(t)
		m.write(".claude/settings.json", settings)
		if _, err := Install(m.options); err == nil || !strings.Contains(err.Error(), "hooks") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestUninstallOnAMachineWithoutFlashheartSaysSo(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	if err := os.RemoveAll(filepath.Join(m.home, ".claude", "skills", "kanban-tracker")); err != nil {
		t.Fatal(err)
	}
	if out := render(m.plan(false)); !strings.Contains(out, "Flashheart is not set up for Claude Code; nothing to remove.") {
		t.Fatalf("plan:\n%s", out)
	}
}

func TestApplyRefusesASettingsFileThatChangedSinceThePlan(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	plan := m.plan(true)
	m.write(".claude/settings.json", string(m.original)+" ")
	if err := plan.Apply(&bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "changed since") {
		t.Fatalf("err = %v", err)
	}
	if m.exists(".claude/skills/flashheart") {
		t.Fatal("a refused apply wrote something")
	}
}

func TestOnlyFlashheartHookCommandsAreOwned(t *testing.T) {
	t.Parallel()

	for command, own := range map[string]bool{
		"/usr/local/bin/flashheart hook claude Stop":     true,
		"'/opt/Flash Heart/flashheart' hook claude Stop": true,
		"flashheart hook claude Stop --root /b":          true,
		"/src/build/flashheart-dev hook claude Stop":     true,
		`echo "flashheart hook fired"`:                   false,
		"notify-send flashheart hook":                    false,
		"/usr/local/bin/flashheart serve":                false,
	} {
		if got := ownsCommand(command); got != own {
			t.Errorf("ownsCommand(%q) = %v, want %v", command, got, own)
		}
	}
}

func TestUninstallRestoresAUsersOwnFlashheartSkill(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.write(".claude/skills/flashheart/SKILL.md", "---\nname: flashheart\n---\nmy own notes\n")
	m.apply(true)
	m.apply(false)
	if got := string(m.read(".claude/skills/flashheart/SKILL.md")); got != "---\nname: flashheart\n---\nmy own notes\n" {
		t.Fatalf("skill after uninstall = %q", got)
	}
}

// A root with spaces in its path is compared as the CLI prints it, so setup
// stays idempotent (FH-8 review).
func TestARootWithSpacesStaysRegistered(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.options.Root = "/Users/me/Library/Mobile Documents/board"
	m.apply(true)
	if plan := m.plan(true); len(plan.commands) != 0 {
		t.Fatalf("second plan re-registers: %v", plan.commands)
	}
}

// A flashheart server registered in another scope is not setup's user-scope
// registration: setup adds its own and removes nothing.
func TestAnotherScopesServerIsLeftAlone(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.server = `{"type": "stdio", "command": "/old/flashheart", "args": ["mcp"]}`
	m.scope = "Local config (private to you in this project)"
	plan := m.plan(true)
	if len(plan.commands) != 1 || plan.commands[0][1] != "add-json" {
		t.Fatalf("commands = %v", plan.commands)
	}
}
