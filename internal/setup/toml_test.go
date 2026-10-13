package setup

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestScanTOMLFindsTablesAndKeys(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "setup", "codex", "user", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := scanTOML(data)
	if err != nil {
		t.Fatal(err)
	}
	var headers, keys []string
	for _, entry := range entries {
		name := strings.Join(entry.path, " / ")
		if entry.header {
			headers = append(headers, name)
		} else {
			keys = append(keys, name+" = "+entry.value)
		}
		if !strings.HasSuffix(string(data[entry.start:entry.end]), "\n") {
			t.Errorf("%s does not end at a line end: %q", name, data[entry.start:entry.end])
		}
	}
	wantHeaders := []string{
		"features", "projects / /Users/example/src/demo", "projects / /Users/example/src/it's here",
		"mcp_servers / docs", "mcp_servers / docs / env", "tui",
		"hooks / state / /Users/example/.codex/config.toml:stop:0:0",
	}
	if !slices.Equal(headers, wantHeaders) {
		t.Fatalf("headers = %q", headers)
	}
	for _, want := range []string{
		`model_reasoning_effort = "medium"`,
		"sandbox_workspace_write / network_access = false",
		"mcp_servers / docs / args = [\n  \"-y\",          # fetch it\n  \"docs-server\",\n]",
		`tui / theme = { name = "dusk", accent = "#ff8800" }`,
	} {
		if !slices.Contains(keys, want) {
			t.Errorf("no key %q in %q", want, keys)
		}
	}
}

func TestTOMLValues(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]string{
		`"plain"`:                      "plain",
		`'C:\literal'`:                 `C:\literal`,
		`"tab\tquote\" back\\ \u00e9"`: "tab\tquote\" back\\ é",
	} {
		if got, ok := parseTOMLString(raw); !ok || got != want {
			t.Errorf("parseTOMLString(%s) = %q, %v", raw, got, ok)
		}
	}
	for _, raw := range []string{`plain`, `"open`, `"""multi"""`, `"a" "b"`} {
		if got, ok := parseTOMLString(raw); ok {
			t.Errorf("parseTOMLString(%s) = %q", raw, got)
		}
	}
	if got, ok := parseTOMLStringArray("[\n  \"mcp\", # the command\n  '--root',\n  \"/a b\",\n]"); !ok || !slices.Equal(got, []string{"mcp", "--root", "/a b"}) {
		t.Errorf("array = %q, %v", got, ok)
	}
	for _, value := range []string{"/opt/Flash Heart/flashheart", `/it's "here"\`, "tab\there", "é"} {
		if got, ok := parseTOMLString(tomlString(value)); !ok || got != value {
			t.Errorf("%q round-trips to %q (%s)", value, got, tomlString(value))
		}
	}
}
