package setup

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/hooks/codex"
	"github.com/rztaylor/flashheart/internal/protocol"
)

// codexMachine is a temporary home whose Codex home (~/.codex) starts as a
// copy of a fixture folder in testdata/setup/codex, or empty.
type codexMachine struct {
	t       *testing.T
	home    string
	options Options
	// original holds the fixture's files by name.
	original map[string][]byte
}

var codexFiles = []string{"config.toml", "hooks.json", "AGENTS.md"}

func newCodexMachine(t *testing.T, fixture string) *codexMachine {
	t.Helper()
	home := t.TempDir()
	m := &codexMachine{t: t, home: home, original: map[string][]byte{}}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if fixture != "" {
		for _, name := range codexFiles {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "setup", "codex", fixture, name))
			if err != nil {
				t.Fatal(err)
			}
			m.original[name] = data
			m.write(name, string(data))
		}
	}
	clock := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	m.options = Options{
		Home: home, Binary: "/opt/Flash Heart/bin/flashheart",
		Now: func() time.Time { clock = clock.Add(time.Second); return clock },
		Run: func(name string, args ...string) ([]byte, error) {
			t.Fatalf("setup codex ran a command: %s %v", name, args)
			return nil, nil
		},
	}
	return m
}

// path is a file in the Codex home.
func (m *codexMachine) path(name string) string {
	home := m.options.CodexHome
	if home == "" {
		home = filepath.Join(m.home, ".codex")
	}
	return filepath.Join(home, name)
}

func (m *codexMachine) write(name, data string) {
	m.t.Helper()
	if err := os.MkdirAll(filepath.Dir(m.path(name)), 0o755); err != nil {
		m.t.Fatal(err)
	}
	if err := os.WriteFile(m.path(name), []byte(data), 0o644); err != nil {
		m.t.Fatal(err)
	}
}

func (m *codexMachine) read(name string) string {
	m.t.Helper()
	data, err := os.ReadFile(m.path(name))
	if err != nil {
		m.t.Fatal(err)
	}
	return string(data)
}

func (m *codexMachine) exists(name string) bool {
	_, err := os.Stat(m.path(name))
	return err == nil
}

// snapshot is every file under the home, by path, for "changed nothing".
func (m *codexMachine) snapshot() map[string]string {
	m.t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(m.home, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		files[path] = string(data)
		return err
	})
	if err != nil {
		m.t.Fatal(err)
	}
	return files
}

func (m *codexMachine) plan(install bool) *Plan {
	m.t.Helper()
	var plan *Plan
	var err error
	if install {
		plan, err = Codex{}.Install(m.options)
	} else {
		plan, err = Codex{}.Uninstall(m.options)
	}
	if err != nil {
		m.t.Fatal(err)
	}
	return plan
}

func (m *codexMachine) apply(install bool) string {
	m.t.Helper()
	var out bytes.Buffer
	if err := m.plan(install).Apply(&out); err != nil {
		m.t.Fatal(err)
	}
	return out.String()
}

// codexHooks reads hooks.json's hook commands by event.
func (m *codexMachine) codexHooks() map[string][]string {
	m.t.Helper()
	var file struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(m.read("hooks.json")), &file); err != nil {
		m.t.Fatal(err)
	}
	commands := map[string][]string{}
	for event, groups := range file.Hooks {
		for _, group := range groups {
			for _, hook := range group.Hooks {
				command := hook.Command
				if group.Matcher != "" {
					command = group.Matcher + " => " + command
				}
				commands[event] = append(commands[event], command)
			}
		}
	}
	return commands
}

// removedLines are the lines a change takes out of a file.
func removedLines(before, after string) []string {
	var removed []string
	for _, line := range strings.Split(unifiedDiff("f", []byte(before), []byte(after)), "\n") {
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "--- ") {
			removed = append(removed, line)
		}
	}
	return removed
}

const quotedBinary = "'/opt/Flash Heart/bin/flashheart'"

func TestCodexInstallShowsADiffAndChangesNothing(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		fixture string
		want    []string
	}{
		{"", []string{
			"Create ~/.codex/config.toml:",
			"Create ~/.codex/hooks.json:",
			"Create ~/.codex/AGENTS.md:",
		}},
		{"user", []string{
			"Change ~/.codex/config.toml:\n--- ~/.codex/config.toml\n+++ ~/.codex/config.toml\n@@ ",
			"Change ~/.codex/hooks.json:",
			"Change ~/.codex/AGENTS.md:",
		}},
		{"stale", []string{
			"-command = \"/Users/example/old/flashheart\"",
			"-          {\"type\": \"command\", \"command\": \"/Users/example/old/flashheart hook codex SessionStart\", \"timeout\": 5},",
			"-An older version of the protocol text.",
		}},
	} {
		t.Run("fixture "+test.fixture, func(t *testing.T) {
			t.Parallel()
			m := newCodexMachine(t, test.fixture)
			before := m.snapshot()
			out := render(m.plan(true))
			want := append(test.want,
				"+[mcp_servers.flashheart]",
				`+command = "/opt/Flash Heart/bin/flashheart"`,
				`+args = ["mcp"]`,
				`"command": "`+quotedBinary+` hook codex SessionStart",`,
				"+"+agentsBegin,
				"+# Flashheart board",
				"/hooks",
			)
			if test.fixture == "stale" {
				// The table, its args and the section's first lines are
				// there already.
				want = slices.DeleteFunc(want, func(part string) bool {
					return slices.Contains([]string{"+[mcp_servers.flashheart]", `+args = ["mcp"]`, "+" + agentsBegin, "+# Flashheart board"}, part)
				})
			}
			for _, part := range want {
				if !strings.Contains(out, part) {
					t.Fatalf("plan missing %q:\n%s", part, out)
				}
			}
			if strings.Contains(out, "+"+`        "matcher": "mcp__flashheart__.*",`) || strings.Contains(out, "+    \"PreToolUse\"") {
				t.Fatalf("plan adds a PreToolUse hook:\n%s", out)
			}
			if after := m.snapshot(); !mapsEqual(before, after) {
				t.Fatal("rendering the plan changed a file")
			}
		})
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if other, found := b[key]; !found || other != value {
			return false
		}
	}
	return true
}

func TestCodexInstallAddsFlashheartAndKeepsTheUsersEntries(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "user")
	out := m.apply(true)
	for _, part := range []string{
		"Wrote ~/.codex/config.toml", "Wrote ~/.codex/hooks.json", "Wrote ~/.codex/AGENTS.md",
		"Backups are in ~/.codex/flashheart-backup/20261012T120001Z-",
		"/hooks", "Start a new Codex session to pick up the changes.",
	} {
		if !strings.Contains(out, part) {
			t.Fatalf("output missing %q:\n%s", part, out)
		}
	}

	// Every file only gains lines: the user's text stays as written.
	for _, name := range codexFiles {
		if removed := removedLines(string(m.original[name]), m.read(name)); len(removed) > 0 {
			t.Errorf("%s lost lines %q", name, removed)
		}
	}
	config := m.read("config.toml")
	if !strings.Contains(config, "[mcp_servers.docs.env]\nDOCS_TOKEN_FILE = '/Users/example/.config/docs\\token'\n\n[mcp_servers.flashheart]\ncommand = \"/opt/Flash Heart/bin/flashheart\"\nargs = [\"mcp\"]\n\n[tui]") {
		t.Fatalf("the server is not beside the other MCP servers:\n%s", config)
	}

	hooks := m.codexHooks()
	if len(hooks) != len(codexEvents) {
		t.Fatalf("hooks for %d events, want %d: %v", len(hooks), len(codexEvents), hooks)
	}
	for _, event := range codexEvents {
		want := quotedBinary + " hook codex " + event
		if !slices.Contains(hooks[event], want) {
			t.Errorf("%s hooks = %q, want %q", event, hooks[event], want)
		}
	}
	if _, found := hooks["PreToolUse"]; found {
		t.Errorf("PreToolUse is registered, which the adapter ignores: %q", hooks["PreToolUse"])
	}
	if !slices.Contains(hooks["Stop"], "afplay /System/Library/Sounds/Glass.aiff") || !slices.Contains(hooks["PostToolUse"], "apply_patch => /Users/example/bin/format-changed") {
		t.Fatalf("the user's hooks are gone: %v", hooks)
	}
	if !strings.Contains(m.read("hooks.json"), "“quoted” ünïcode") {
		t.Fatal("hooks.json lost its description")
	}

	agents := m.read("AGENTS.md")
	if want := string(m.original["AGENTS.md"]) + "\n" + agentsBegin + "\n" + protocol.Body() + agentsEnd + "\n"; agents != want {
		t.Fatalf("AGENTS.md:\n%s", unifiedDiff("AGENTS.md", []byte(want), []byte(agents)))
	}

	backups, _ := filepath.Glob(m.path(filepath.Join("flashheart-backup", "*", "config.toml")))
	if len(backups) != 1 {
		t.Fatalf("backups = %v", backups)
	}
	if saved, _ := os.ReadFile(backups[0]); !bytes.Equal(saved, m.original["config.toml"]) {
		t.Fatal("the backup is not the original config.toml")
	}
	if again := m.plan(true); !again.Empty() {
		t.Fatalf("a second install is not empty:\n%s", render(again))
	}
	if out := render(m.plan(true)); out != "Codex is already configured for Flashheart; nothing to change.\n" {
		t.Fatalf("second plan:\n%s", out)
	}
}

func TestCodexInstallReplacesAStaleSetup(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "stale")
	m.apply(true)
	config := m.read("config.toml")
	for _, part := range []string{
		"[mcp_servers.flashheart]\ncommand = \"/opt/Flash Heart/bin/flashheart\"\nargs = [\"mcp\"]\nstartup_timeout_sec = 30 # mine, kept\n\n[mcp_servers.flashheart.env]\nFLASHHEART_DEBUG = \"1\"\n",
		"[projects.\"/Users/example/src/demo\"]\ntrust_level = \"trusted\"\n",
		"[tui]\ntheme = \"dusk\"\n",
	} {
		if !strings.Contains(config, part) {
			t.Fatalf("config.toml lacks %q:\n%s", part, config)
		}
	}
	if strings.Contains(config+m.read("hooks.json"), "/Users/example/old") {
		t.Fatal("the old binary is still configured")
	}
	hooks := m.codexHooks()
	if !slices.Contains(hooks["SessionStart"], "echo session started") || strings.Count(m.read("hooks.json"), "hook codex SessionStart") != 1 {
		t.Fatalf("SessionStart hooks = %q", hooks["SessionStart"])
	}
	// The PreToolUse hook an earlier setup wrote goes, event and all.
	if strings.Contains(m.read("hooks.json"), "PreToolUse") {
		t.Fatalf("the earlier PreToolUse hook remains:\n%s", m.read("hooks.json"))
	}
	agents := m.read("AGENTS.md")
	if strings.Contains(agents, "An older version") || strings.Count(agents, agentsBegin) != 1 ||
		!strings.HasPrefix(agents, "# My global rules\n\n- Answer in British English.\n\n"+agentsBegin+"\n# Flashheart board\n") ||
		!strings.HasSuffix(agents, agentsEnd+"\n\n## More of my rules\n\n- Keep commits small.\n") {
		t.Fatalf("AGENTS.md:\n%s", agents)
	}
	if again := m.plan(true); !again.Empty() {
		t.Fatalf("a second install is not empty:\n%s", render(again))
	}
}

func TestCodexUninstallPutsBackTheUsersFiles(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "user")
	m.apply(true)
	out := render(m.plan(false))
	for _, part := range []string{"Change ~/.codex/config.toml:", "-[mcp_servers.flashheart]", "Change ~/.codex/hooks.json:", "Change ~/.codex/AGENTS.md:", "-" + agentsBegin} {
		if !strings.Contains(out, part) {
			t.Fatalf("uninstall plan missing %q:\n%s", part, out)
		}
	}
	m.apply(false)
	for _, name := range codexFiles {
		if got := m.read(name); got != string(m.original[name]) {
			t.Errorf("%s after uninstall:\n%s", name, unifiedDiff(name, m.original[name], []byte(got)))
		}
	}
	if again := m.plan(false); !again.Empty() {
		t.Fatalf("a second uninstall is not empty:\n%s", render(again))
	}
	if out := render(m.plan(false)); out != "Flashheart is not set up for Codex; nothing to remove.\n" {
		t.Fatalf("second uninstall plan:\n%s", out)
	}
}

// Codex writes project and hook trust into config.toml itself, and the
// user edits their own hooks: uninstalling takes out only Flashheart's.
func TestCodexUninstallAfterChangesRemovesOnlyFlashheart(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "user")
	m.apply(true)
	trust := "\n[projects.\"/Users/example/src/new\"]\ntrust_level = \"trusted\"\n\n[hooks.state.\"" + m.path("hooks.json") + ":Stop:0:0\"]\ntrusted_hash = \"sha256:1111\"\n"
	m.write("config.toml", m.read("config.toml")+trust)
	m.write("hooks.json", strings.Replace(m.read("hooks.json"), "afplay /System/Library/Sounds/Glass.aiff", "say done", 1))
	m.apply(false)

	if want := string(m.original["config.toml"]) + trust; m.read("config.toml") != want {
		t.Fatalf("config.toml:\n%s", unifiedDiff("config.toml", []byte(want), []byte(m.read("config.toml"))))
	}
	hooks := m.read("hooks.json")
	if strings.Contains(hooks, "flashheart") || !strings.Contains(hooks, "say done") || !strings.Contains(hooks, "format-changed") {
		t.Fatalf("hooks.json:\n%s", hooks)
	}
	if m.read("AGENTS.md") != string(m.original["AGENTS.md"]) {
		t.Fatalf("AGENTS.md:\n%s", m.read("AGENTS.md"))
	}
}

func TestCodexUninstallRemovesAStaleSetup(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "stale")
	m.apply(false)
	if want := "model = \"gpt-6-astra\"\n\n[projects.\"/Users/example/src/demo\"]\ntrust_level = \"trusted\"\n\n[tui]\ntheme = \"dusk\"\n"; m.read("config.toml") != want {
		t.Fatalf("config.toml:\n%s", m.read("config.toml"))
	}
	hooks := m.read("hooks.json")
	if strings.Contains(hooks, "flashheart") || !strings.Contains(hooks, "echo session started") || strings.Contains(hooks, "Stop") {
		t.Fatalf("hooks.json:\n%s", hooks)
	}
	if want := "# My global rules\n\n- Answer in British English.\n\n## More of my rules\n\n- Keep commits small.\n"; m.read("AGENTS.md") != want {
		t.Fatalf("AGENTS.md:\n%q", m.read("AGENTS.md"))
	}
}

func TestCodexInstallOnAFreshCodexHomeAndBack(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "")
	m.apply(true)
	if want := "[mcp_servers.flashheart]\ncommand = \"/opt/Flash Heart/bin/flashheart\"\nargs = [\"mcp\"]\n"; m.read("config.toml") != want {
		t.Fatalf("config.toml = %q", m.read("config.toml"))
	}
	if want := agentsBegin + "\n" + protocol.Body() + agentsEnd + "\n"; m.read("AGENTS.md") != want {
		t.Fatal("AGENTS.md is not just the protocol section")
	}
	if len(m.codexHooks()) != len(codexEvents) {
		t.Fatalf("hooks = %v", m.codexHooks())
	}
	m.apply(false)
	for _, name := range codexFiles {
		if m.exists(name) {
			t.Errorf("%s did not exist before setup but remains:\n%s", name, m.read(name))
		}
	}
}

// An empty AGENTS.md (Codex's app creates one) stays, empty.
func TestCodexUninstallKeepsAnEmptyAgentsFileItFound(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "")
	m.write("AGENTS.md", "")
	m.apply(true)
	m.apply(false)
	if !m.exists("AGENTS.md") || m.read("AGENTS.md") != "" {
		t.Fatal("the empty AGENTS.md was not kept")
	}
}

func TestCodexSetupHonoursCodexHome(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "")
	m.options.CodexHome = filepath.Join(m.home, "elsewhere", "codex")
	out := render(m.plan(true))
	if !strings.Contains(out, "Create ~/elsewhere/codex/config.toml:") {
		t.Fatalf("plan:\n%s", out)
	}
	m.apply(true)
	if !m.exists("config.toml") || !m.exists("flashheart-backup") {
		t.Fatal("nothing written to $CODEX_HOME")
	}
	if entries, _ := os.ReadDir(filepath.Join(m.home, ".codex")); len(entries) != 0 {
		t.Fatalf("~/.codex was written: %v", entries)
	}
}

func TestCodexSetupPassesANonDefaultRoot(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "user")
	m.options.Root = `/Users/me/Library/Mobile Documents/"board"`
	m.apply(true)
	config := m.read("config.toml")
	if !strings.Contains(config, `args = ["mcp", "--root", "/Users/me/Library/Mobile Documents/\"board\""]`) {
		t.Fatalf("config.toml:\n%s", config)
	}
	if hooks := m.codexHooks(); !slices.Contains(hooks["Stop"], quotedBinary+` hook codex Stop --root '/Users/me/Library/Mobile Documents/"board"'`) {
		t.Fatalf("Stop hooks = %q", hooks["Stop"])
	}
	if again := m.plan(true); !again.Empty() {
		t.Fatalf("a second install is not empty:\n%s", render(again))
	}
}

func TestCodexSetupRefusesFilesItCannotEditSafely(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, file, data, want string
	}{
		{"dotted server keys", "config.toml", "mcp_servers.flashheart.command = \"/x/flashheart\"\n", "mcp_servers.flashheart"},
		{"inline server table", "config.toml", "[mcp_servers]\nflashheart = { command = \"/x/flashheart\" }\n", "mcp_servers.flashheart"},
		{"inline servers", "config.toml", "mcp_servers = { docs = { command = \"npx\" } }\n", "mcp_servers"},
		{"two server tables", "config.toml", "[mcp_servers.flashheart]\ncommand = \"a\"\n[mcp_servers.flashheart]\ncommand = \"b\"\n", "mcp_servers.flashheart"},
		{"unterminated string", "config.toml", "model = \"gpt\n", "config.toml"},
		{"unterminated array", "config.toml", "notify = [\"a\",\n", "config.toml"},
		{"hooks not an object", "hooks.json", `{"hooks": []}`, "hooks"},
		{"hooks.json not JSON", "hooks.json", "{ nope", "hooks.json"},
		{"begin without end", "AGENTS.md", "# Mine\n\n" + agentsBegin + "\nold\n", "AGENTS.md"},
		{"two sections", "AGENTS.md", agentsBegin + "\n" + agentsEnd + "\n" + agentsBegin + "\n" + agentsEnd + "\n", "AGENTS.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			m := newCodexMachine(t, "")
			m.write(test.file, test.data)
			for _, install := range []bool{true, false} {
				var err error
				if install {
					_, err = Codex{}.Install(m.options)
				} else {
					_, err = Codex{}.Uninstall(m.options)
				}
				if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "fix it") {
					t.Errorf("install %v: err = %v", install, err)
				}
			}
		})
	}
}

// Codex saves trust in config.toml while it runs: a plan made before that
// would undo it, so applying it is refused.
func TestCodexApplyRefusesAConfigThatChangedSinceThePlan(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "user")
	plan := m.plan(true)
	m.write("config.toml", string(m.original["config.toml"])+"\n[projects.\"/x\"]\ntrust_level = \"trusted\"\n")
	if err := plan.Apply(&bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "changed since") {
		t.Fatalf("err = %v", err)
	}
	if m.read("AGENTS.md") != string(m.original["AGENTS.md"]) {
		t.Fatal("a refused apply wrote something")
	}
}

// Setup registers every event the Codex hook adapter has recorded payloads
// for (testdata/hooks/codex), less those the adapter ignores, and no other.
func TestCodexHooksAreTheRecordedEvents(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(filepath.Join("..", "..", "testdata", "hooks", "codex"))
	if err != nil {
		t.Fatal(err)
	}
	var recorded []string
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "mcp" {
			recorded = append(recorded, entry.Name())
		}
	}
	registered := slices.Concat(codexEvents, codexIgnoredEvents)
	slices.Sort(recorded)
	slices.Sort(registered)
	if !slices.Equal(recorded, registered) {
		t.Fatalf("codexEvents = %v, recorded events = %v", registered, recorded)
	}
}

// An event setup leaves out is one the adapter does nothing with: every
// recorded payload of it gives no events and nothing to say back. When the
// adapter starts handling one, this fails until setup registers it.
func TestCodexIgnoredEventsAreIgnoredByTheAdapter(t *testing.T) {
	t.Parallel()

	for _, event := range codexIgnoredEvents {
		if slices.Contains(codexEvents, event) {
			t.Errorf("%s is both registered and ignored", event)
		}
		payloads, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "hooks", "codex", event, "*.json"))
		checked := 0
		for _, path := range payloads {
			if strings.HasSuffix(path, ".events.json") || strings.HasSuffix(path, ".output.json") {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			input, err := codex.Adapter{}.Parse(event, data)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			if len(input.Events) > 0 || input.Recovery || input.Inbox != "" || input.Stop || input.Reply != nil || input.Settled {
				t.Errorf("%s: the adapter acts on %s (%+v); register it in codexEvents", path, event, input)
			}
			checked++
		}
		if checked == 0 {
			t.Errorf("no recorded %s payloads", event)
		}
	}
}
