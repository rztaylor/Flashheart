package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installed is a machine set up by a flashheart binary that exists.
func installed(t *testing.T) *machine {
	t.Helper()
	m := newMachine(t)
	m.options.Binary = binary(t, m.home, "bin/flashheart")
	m.apply(true)
	return m
}

func binary(t *testing.T, home, name string) string {
	t.Helper()
	path := filepath.Join(home, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func diagnose(t *testing.T, o Options) (problems, warnings, ok []string) {
	t.Helper()
	for _, finding := range Diagnose(o) {
		switch finding.Level {
		case Problem:
			problems = append(problems, finding.Text)
		case Warning:
			warnings = append(warnings, finding.Text)
		default:
			ok = append(ok, finding.Text)
		}
	}
	return problems, warnings, ok
}

func TestDiagnoseAHealthySetup(t *testing.T) {
	t.Parallel()

	m := installed(t)
	problems, warnings, ok := diagnose(t, m.options)
	if len(problems)+len(warnings) != 0 {
		t.Fatalf("problems %q, warnings %q", problems, warnings)
	}
	if want := []string{"Hooks: all 16 installed", "MCP server: registered", "Skill: up to date"}; strings.Join(ok, "|") != strings.Join(want, "|") {
		t.Fatalf("ok = %q", ok)
	}
}

func TestDiagnoseAMissingHook(t *testing.T) {
	t.Parallel()

	m := installed(t)
	settings := string(m.read(".claude/settings.json"))
	command := `"` + m.options.hookCommand("PostToolUse") + `"`
	if !strings.Contains(settings, command) {
		t.Fatalf("no PostToolUse hook in %s", settings)
	}
	m.write(".claude/settings.json", strings.Replace(settings, command, `"say done"`, 1))
	problems, _, _ := diagnose(t, m.options)
	if len(problems) != 1 || !strings.Contains(problems[0], "missing for PostToolUse") || !strings.Contains(problems[0], "flashheart setup claude --write") {
		t.Fatalf("problems = %q", problems)
	}
}

func TestDiagnoseAStaleBinaryPath(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	old := filepath.Join(m.home, "old", "flashheart")
	m.options.Binary = old
	m.apply(true)
	// The binary moved; this one runs doctor.
	m.options.Binary = binary(t, m.home, "bin/flashheart")
	problems, _, _ := diagnose(t, m.options)
	if len(problems) != 2 {
		t.Fatalf("problems = %q", problems)
	}
	for index, area := range []string{"Hooks", "MCP server"} {
		if !strings.HasPrefix(problems[index], area+": ") || !strings.Contains(problems[index], old+", which does not exist") {
			t.Fatalf("problem %d = %q", index, problems[index])
		}
	}

	// A different binary that exists is a warning, not a problem.
	binary(t, m.home, "old/flashheart")
	problems, warnings, _ := diagnose(t, m.options)
	if len(problems) != 0 || len(warnings) != 2 || !strings.Contains(warnings[0], "not this binary") {
		t.Fatalf("problems %q, warnings %q", problems, warnings)
	}
}

func TestDiagnoseAnUnconfiguredMachine(t *testing.T) {
	t.Parallel()

	m := newMachine(t)
	m.options.Binary = binary(t, m.home, "bin/flashheart")
	problems, warnings, _ := diagnose(t, m.options)
	if len(problems) != 3 {
		t.Fatalf("problems = %q", problems)
	}
	if !strings.Contains(problems[0], "none installed") || !strings.Contains(problems[1], "not registered") || !strings.Contains(problems[2], "not installed") {
		t.Fatalf("problems = %q", problems)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "kanban-tracker") {
		t.Fatalf("warnings = %q", warnings)
	}
}

func TestDiagnoseAnOutdatedSkillAndUnreadableSettings(t *testing.T) {
	t.Parallel()

	m := installed(t)
	m.write(".claude/skills/flashheart/SKILL.md", "---\nname: flashheart\n---\nold text\n")
	_, warnings, _ := diagnose(t, m.options)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "out of date") {
		t.Fatalf("warnings = %q", warnings)
	}

	m.write(".claude/settings.json", "{ not json")
	problems, _, _ := diagnose(t, m.options)
	if len(problems) != 1 || !strings.HasPrefix(problems[0], "Hooks: ") {
		t.Fatalf("problems = %q", problems)
	}
}
