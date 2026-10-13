package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/health"
)

const codexFix = "flashheart setup codex --write"

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

// trust records hook trust as Codex's /hooks does.
func (m *codexMachine) trust() {
	m.write("config.toml", m.read("config.toml")+"\n[hooks.state.\""+m.path("hooks.json")+":SessionStart:0:0\"]\ntrusted_hash = \"sha256:2222\"\n")
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
