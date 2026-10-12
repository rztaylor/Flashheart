package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The engine names no agent: only the agents' own files may (FH-89).
func TestTheEngineNamesNoAgent(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"agent.go", "plan.go", "diff.go", "json.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(data)), "claude") {
			t.Errorf("%s mentions claude; agent specifics belong in the agent's own files", name)
		}
	}
}

// A plan speaks of the agent it configures, whichever that is.
func TestAPlanRendersAndAppliesForAnyAgent(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	clock := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	var ran []string
	options := Options{Home: home, Now: func() time.Time { return clock }, CLI: "/usr/bin/other",
		Run: func(name string, args ...string) ([]byte, error) {
			ran = append(ran, name+" "+strings.Join(args, " "))
			return nil, nil
		}}
	agent := target{title: "Other Agent", command: "other", backups: filepath.Join(home, ".other", "backups"), options: options}

	var out bytes.Buffer
	newPlan(agent, false).Render(&out)
	newPlan(agent, true).Render(&out)
	plan := newPlan(agent, false)
	config := filepath.Join(home, ".other", "config.toml")
	plan.files = append(plan.files, change{path: config, backup: "config.toml", after: []byte("[mcp]\n")})
	plan.commands = append(plan.commands, []string{"mcp", "add", "flashheart"})
	plan.Render(&out)
	if err := plan.Apply(&out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"Other Agent is already configured for Flashheart; nothing to change.",
		"Flashheart is not set up for Other Agent; nothing to remove.",
		"Create ~/.other/config.toml:",
		"Run: other mcp add flashheart",
		"Backups are in ~/.other/backups/20261011T120000Z-",
		"Ran other mcp add flashheart",
		"Start a new Other Agent session to pick up the changes.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(strings.ToLower(text), "claude") {
		t.Errorf("output names Claude:\n%s", text)
	}
	if len(ran) != 1 || ran[0] != "/usr/bin/other mcp add flashheart" {
		t.Errorf("ran %q", ran)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(config), "backups")); err != nil {
		t.Errorf("no backup folder: %v", err)
	}
}
