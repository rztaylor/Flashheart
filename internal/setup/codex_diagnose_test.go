package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/health"
)

const codexFix = "flashheart setup codex"

func diagnoseCodex(t *testing.T, o Options) (problems, warnings, ok []string) {
	t.Helper()
	for _, finding := range (Codex{}).Diagnose(o) {
		switch finding.Level {
		case health.Problem:
			problems = append(problems, finding.Text)
		case health.Warning:
			warnings = append(warnings, finding.Text)
		default:
			ok = append(ok, finding.Text)
		}
	}
	return problems, warnings, ok
}

// installedCodex is a Codex home set up by a flashheart binary that exists,
// with its hooks trusted in Codex.
func installedCodex(t *testing.T, fixture string) *codexMachine {
	t.Helper()
	m := newCodexMachine(t, fixture)
	m.options.Binary = binary(t, m.home, "bin/flashheart")
	m.apply(true)
	m.trust()
	return m
}

// trust records trust in every Flashheart hook in hooks.json as Codex's
// /hooks does, with the key and hash checked against Codex's own
// (TestCodexHookTrustMatchesRecording).
func (m *codexMachine) trust() {
	m.t.Helper()
	tree, err := parseJSON([]byte(m.read("hooks.json")))
	if err != nil {
		m.t.Fatal(err)
	}
	config := m.read("config.toml")
	for _, event := range tree.get("hooks").object {
		for group, groupValue := range event.value.array {
			for handler, entry := range groupValue.get("hooks").array {
				if ownsCommand(entry.get("command").text()) {
					key := codexHookKey(m.path("hooks.json"), event.key, group, handler)
					config += fmt.Sprintf("\n[hooks.state.%s]\ntrusted_hash = %s\n", tomlString(key), tomlString(codexHookHash(event.key, groupValue, entry)))
				}
			}
		}
	}
	m.write("config.toml", config)
}

func TestDiagnoseCodexHealthy(t *testing.T) {
	t.Parallel()

	for _, fixture := range []string{"", "user"} {
		m := installedCodex(t, fixture)
		problems, warnings, ok := diagnoseCodex(t, m.options)
		if len(problems)+len(warnings) != 0 {
			t.Fatalf("%q: problems %q, warnings %q", fixture, problems, warnings)
		}
		want := []string{"MCP server: configured", "Hooks: all 11 installed", "Protocol text: up to date"}
		if len(ok) != len(want) {
			t.Fatalf("%q: ok = %q", fixture, ok)
		}
		for index := range want {
			if !strings.HasPrefix(ok[index], want[index]) {
				t.Fatalf("%q: ok = %q", fixture, ok)
			}
		}
	}
}

func TestDiagnoseCodexUnconfigured(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "user")
	m.options.Binary = binary(t, m.home, "bin/flashheart")
	problems, warnings, _ := diagnoseCodex(t, m.options)
	if len(problems) != 3 || len(warnings) != 0 {
		t.Fatalf("problems %q, warnings %q", problems, warnings)
	}
	for index, want := range []string{"MCP server: not in ~/.codex/config.toml", "Hooks: none installed", "Protocol text: not in ~/.codex/AGENTS.md"} {
		if !strings.HasPrefix(problems[index], want) || !strings.Contains(problems[index], codexFix) {
			t.Errorf("problem %d = %q, want %q with the fix", index, problems[index], want)
		}
	}
}

// Without a Codex home there is no Codex to check: doctor stays quiet for
// people who use only Claude Code.
func TestDiagnoseCodexNotInstalled(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "")
	if err := os.Remove(filepath.Join(m.home, ".codex")); err != nil {
		t.Fatal(err)
	}
	problems, warnings, ok := diagnoseCodex(t, m.options)
	if len(problems)+len(warnings) != 0 || len(ok) != 1 || !strings.Contains(ok[0], "not installed") || !strings.Contains(ok[0], codexFix) {
		t.Fatalf("problems %q, warnings %q, ok %q", problems, warnings, ok)
	}
}

func TestDiagnoseCodexStaleSetup(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "stale")
	m.options.Binary = binary(t, m.home, "bin/flashheart")
	m.trust()
	old := "/Users/example/old/flashheart"
	problems, warnings, _ := diagnoseCodex(t, m.options)
	for _, want := range []string{
		"MCP server: runs " + old + ", which does not exist",
		"Hooks: missing for UserPromptSubmit, PermissionRequest",
		"Hooks: run " + old + ", which does not exist",
	} {
		if !containsPrefix(problems, want) {
			t.Errorf("no problem %q in %q", want, problems)
		}
	}
	for _, problem := range problems {
		if !strings.Contains(problem, codexFix) {
			t.Errorf("problem without the fix: %q", problem)
		}
	}
	if len(warnings) != 2 || !strings.HasPrefix(warnings[0], "Hooks: also installed for PreToolUse, which setup no longer registers") ||
		!strings.HasPrefix(warnings[1], "Protocol text: out of date") || !strings.Contains(warnings[0], codexFix) || !strings.Contains(warnings[1], codexFix) {
		t.Fatalf("warnings = %q", warnings)
	}

	// After setup, a binary that moved but still exists is a warning.
	m.apply(true)
	m.trust()
	m.options.Binary = binary(t, m.home, "new/flashheart")
	problems, warnings, _ = diagnoseCodex(t, m.options)
	if len(problems) != 0 || len(warnings) != 2 || !strings.Contains(warnings[0], "not this binary") || !strings.Contains(warnings[1], "not this binary") {
		t.Fatalf("problems %q, warnings %q", problems, warnings)
	}
}

func TestDiagnoseCodexServerArgumentsAndBrokenFiles(t *testing.T) {
	t.Parallel()

	m := installedCodex(t, "user")
	m.write("config.toml", strings.Replace(m.read("config.toml"), `args = ["mcp"]`, `args = ["mcp", "--root", "/elsewhere"]`, 1))
	_, warnings, _ := diagnoseCodex(t, m.options)
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], "MCP server: runs with \"mcp --root /elsewhere\"") {
		t.Fatalf("warnings = %q", warnings)
	}

	m.write("config.toml", "model = \"unterminated\n")
	m.write("hooks.json", "{ nope")
	m.write("AGENTS.md", agentsBegin+"\nno end\n")
	problems, _, _ := diagnoseCodex(t, m.options)
	if len(problems) != 3 || !strings.HasPrefix(problems[0], "MCP server: cannot read") || !strings.HasPrefix(problems[1], "Hooks: cannot read") || !strings.HasPrefix(problems[2], "Protocol text: cannot read") {
		t.Fatalf("problems = %q", problems)
	}
}

// Codex skips hooks nobody trusted; with no trust recorded for hooks.json
// at all, doctor says how to trust them.
func TestDiagnoseCodexUntrustedHooks(t *testing.T) {
	t.Parallel()

	m := newCodexMachine(t, "user")
	m.options.Binary = binary(t, m.home, "bin/flashheart")
	m.apply(true)
	problems, warnings, _ := diagnoseCodex(t, m.options)
	if len(problems) != 0 || len(warnings) != 1 || !strings.HasPrefix(warnings[0], "Hooks: not trusted yet") || !strings.Contains(warnings[0], "/hooks") {
		t.Fatalf("problems %q, warnings %q", problems, warnings)
	}
}

// recordedTrust is a Codex home holding the trust Codex saved for
// Flashheart's hooks (testdata/setup/codex/trusted): the recorded
// config.toml, its keys moved to this home (keys are not hashed), and
// hooksFixture as hooks.json. Codex keys trust by $CODEX_HOME's real path.
func recordedTrust(t *testing.T, hooksFixture string, viaLink bool) *codexMachine {
	t.Helper()
	m := newCodexMachine(t, "")
	m.options.Binary = "/Users/example/bin/flashheart"
	m.options.Root = "/Users/example/reports/Kanban"
	real, err := filepath.EvalSymlinks(filepath.Join(m.home, ".codex"))
	if err != nil {
		t.Fatal(err)
	}
	if viaLink {
		m.options.CodexHome = filepath.Join(m.home, "link")
		if err := os.Symlink(real, m.options.CodexHome); err != nil {
			t.Fatal(err)
		}
	}
	m.write("hooks.json", trustedFixture(t, hooksFixture))
	m.write("config.toml", strings.ReplaceAll(trustedFixture(t, "config.toml"), recordedCodexHome+"/", real+"/"))
	return m
}

// trustWarnings are doctor's warnings about hooks Codex will skip.
func trustWarnings(t *testing.T, o Options) []string {
	t.Helper()
	_, warnings, _ := diagnoseCodex(t, o)
	var trust []string
	for _, warning := range warnings {
		if strings.Contains(warning, "/hooks") {
			trust = append(trust, warning)
		}
	}
	return trust
}

// Doctor compares the trust Codex saved with Flashheart's hooks as they
// are now and names the events whose hook Codex will skip: never trusted,
// or trusted when its command was different (a moved binary, another
// --root). The user's own hooks are not Flashheart's business.
func TestDiagnoseCodexHookTrust(t *testing.T) {
	t.Parallel()

	allEvents := strings.Join(codexEvents, ", ")
	for _, test := range []struct {
		name    string
		hooks   string
		viaLink bool
		edit    func(m *codexMachine)
		want    []string
	}{
		{name: "trusted", hooks: "hooks.json"},
		{name: "trusted through a linked CODEX_HOME", hooks: "hooks.json", viaLink: true},
		{
			name:  "never trusted",
			hooks: "hooks.json",
			edit: func(m *codexMachine) {
				config := m.read("config.toml")
				m.write("config.toml", config[:strings.Index(config, "[hooks.state]")])
			},
			want: []string{"Hooks: not trusted yet, so Codex skips them; start codex, type /hooks and trust each Flashheart hook"},
		},
		{
			name:  "some untrusted",
			hooks: "hooks.json",
			edit: func(m *codexMachine) {
				// Drop two tables: a header and its trusted_hash line each.
				lines := strings.Split(m.read("config.toml"), "\n")
				for _, key := range []string{":session_start:0:0\"]", ":stop:1:0\"]"} {
					index := slices.IndexFunc(lines, func(line string) bool { return strings.HasSuffix(line, key) })
					lines = slices.Delete(lines, index, index+2)
				}
				m.write("config.toml", strings.Join(lines, "\n"))
			},
			want: []string{"Hooks: not trusted for SessionStart, Stop, so Codex skips them; start codex, type /hooks and trust them"},
		},
		{
			name:  "stale after a command change",
			hooks: "hooks-moved.json",
			want:  []string{"Hooks: changed since you trusted them for " + allEvents + ", so Codex skips them; start codex, type /hooks and trust them again"},
		},
		{
			name:  "one stale",
			hooks: "hooks.json",
			edit: func(m *codexMachine) {
				m.write("hooks.json", strings.Replace(m.read("hooks.json"), "codex SessionStart --root /Users/example/reports/Kanban", "codex SessionStart --root /Users/example/other-board", 1))
			},
			want: []string{"Hooks: changed since you trusted them for SessionStart, so Codex skips them; start codex, type /hooks and trust them again"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			m := recordedTrust(t, test.hooks, test.viaLink)
			if test.edit != nil {
				test.edit(m)
			}
			got := trustWarnings(t, m.options)
			if strings.Join(got, "\n") != strings.Join(test.want, "\n") {
				t.Fatalf("trust warnings:\n%q\nwant:\n%q", got, test.want)
			}
		})
	}
}

func TestDiagnoseCodexHooksTurnedOff(t *testing.T) {
	t.Parallel()

	m := installedCodex(t, "user")
	m.write("config.toml", strings.Replace(m.read("config.toml"), "[features]\n", "[features]\nhooks = false\n", 1))
	_, warnings, _ := diagnoseCodex(t, m.options)
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], "Hooks: turned off") {
		t.Fatalf("warnings = %q", warnings)
	}
}

func containsPrefix(list []string, prefix string) bool {
	for _, item := range list {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}
