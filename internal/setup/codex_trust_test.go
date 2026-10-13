package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// trustedFixture is a file of testdata/setup/codex/trusted, recorded from
// Codex by scripts/record-codex-hook-trust.sh (testdata/hooks/codex/MANIFEST.md).
func trustedFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "setup", "codex", "trusted", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// recordedCodexHome is the Codex home in the recordings' keys.
const recordedCodexHome = "/Users/example/.codex"

// Codex's own key and hash for every hook it listed match what doctor
// computes from the same hooks.json: Flashheart's hooks as setup writes
// them, after their commands change, and hooks using every field that is
// hashed (matcher, timeout, async, statusMessage, additionalContextLimit)
// with text JSON encoders escape differently.
func TestCodexHookTrustMatchesRecording(t *testing.T) {
	t.Parallel()

	for _, recording := range []struct{ hooks, list string }{
		{"hooks.json", "hooks-list-untrusted.json"},
		{"hooks-moved.json", "hooks-list-moved.json"},
		{"hooks-edge.json", "hooks-list-edge.json"},
	} {
		tree, err := parseJSON([]byte(trustedFixture(t, recording.hooks)))
		if err != nil {
			t.Fatal(err)
		}
		var listed []struct{ Key, CurrentHash string }
		if err := json.Unmarshal([]byte(trustedFixture(t, recording.list)), &listed); err != nil {
			t.Fatal(err)
		}
		if len(listed) == 0 {
			t.Fatalf("%s: no hooks", recording.list)
		}
		hooksFile := filepath.Join(recordedCodexHome, "hooks.json")
		for _, want := range listed {
			parts := strings.Split(strings.TrimPrefix(want.Key, hooksFile+":"), ":")
			if len(parts) != 3 {
				t.Fatalf("%s: key %q", recording.list, want.Key)
			}
			group, _ := strconv.Atoi(parts[1])
			handler, _ := strconv.Atoi(parts[2])
			event := ""
			for _, member := range tree.get("hooks").object {
				if codexEventLabel(member.key) == parts[0] {
					event = member.key
				}
			}
			if event == "" {
				t.Fatalf("%s: no event for %q", recording.list, want.Key)
			}
			if key := codexHookKey(hooksFile, event, group, handler); key != want.Key {
				t.Errorf("%s: key %q, Codex has %q", recording.list, key, want.Key)
			}
			groupValue := tree.get("hooks").get(event).array[group]
			if hash := codexHookHash(event, groupValue, groupValue.get("hooks").array[handler]); hash != want.CurrentHash {
				t.Errorf("%s: %s hashes to %q, Codex has %q", recording.list, want.Key, hash, want.CurrentHash)
			}
		}
	}
}

func TestCodexHookHashUnknownShapes(t *testing.T) {
	t.Parallel()

	for _, handler := range []string{
		`{"type": "mcp_tool", "server": "s", "tool": "t"}`,
		`{"type": "command", "command": "x", "timeout": -1}`,
		`{"type": "command", "command": "x", "timeout": 1.5}`,
		`{"type": "command", "command": 7}`,
		`{"type": "command", "command": "x", "async": "yes"}`,
	} {
		tree, err := parseJSON([]byte(handler))
		if err != nil {
			t.Fatal(err)
		}
		if hash := codexHookHash("Stop", &value{isObj: true}, tree); hash != "" {
			t.Errorf("%s hashes to %q, want none", handler, hash)
		}
	}
}
