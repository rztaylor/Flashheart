package events

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRunIDsFollowTheGrammar(t *testing.T) {
	t.Parallel()

	cases := []struct {
		id                         string
		agent, session, sub, short string
		subagent                   bool
	}{
		{"claude:6f1c2d3e-aaaa-bbbb", "claude", "6f1c2d3e-aaaa-bbbb", "", "claude:6f1c2d3e", false},
		{"claude:6f1c2d3e-aaaa/a1b2c3", "claude", "6f1c2d3e-aaaa", "a1b2c3", "claude:6f1c2d3e", true},
		{"codex:mcp-9bd225d4ac50", "codex", "mcp-9bd225d4ac50", "", "codex:mcp-9bd2", false},
		{"my-agent2:s_1.x", "my-agent2", "s_1.x", "", "my-agent2:s_1.x", false},
	}
	for _, tc := range cases {
		r, err := ParseRunID(tc.id)
		if err != nil {
			t.Errorf("%s: %v", tc.id, err)
			continue
		}
		if r.String() != tc.id || r.Agent() != tc.agent || r.SessionID() != tc.session || r.SubagentID() != tc.sub || r.IsSubagent() != tc.subagent || r.Short() != tc.short {
			t.Errorf("%s: parsed as %q agent %q session %q sub %q short %q", tc.id, r, r.Agent(), r.SessionID(), r.SubagentID(), r.Short())
		}
		if session := r.Session(); session.IsSubagent() || session.String() != tc.agent+":"+tc.session {
			t.Errorf("%s: session %q", tc.id, session)
		}
	}

	built, err := SessionRun("claude", "abc")
	if err != nil {
		t.Fatal(err)
	}
	sub, err := built.Subagent("x1")
	if err != nil || sub.String() != "claude:abc/x1" || !sub.IsSubagent() || sub.Session() != built {
		t.Fatalf("built %q, %v", sub, err)
	}
}

func TestInvalidRunIDsAreRefused(t *testing.T) {
	t.Parallel()

	for _, id := range []string{
		"", "claude", "claude:", ":abc", "Claude:abc", "1claude:abc", "claude:abc/", "claude:a b",
		"claude:abc/x/y", "claude:a:b", "claude:abc/x:y", "claude:a/../b",
		"agent-name-that-is-far-too-long-for-the-grammar:abc",
	} {
		if r, err := ParseRunID(id); !errors.Is(err, ErrInvalidRunID) {
			t.Errorf("%q parsed as %q, %v", id, r, err)
		}
	}
	if _, err := SessionRun("claude", "a/b"); !errors.Is(err, ErrInvalidRunID) {
		t.Errorf("session with a slash: %v", err)
	}
	if _, err := (RunID{agent: "claude", session: "abc"}).Subagent("a:b"); !errors.Is(err, ErrInvalidRunID) {
		t.Errorf("subagent with a colon: %v", err)
	}
}

// Recorded ids are read leniently: the human's runs and malformed ids from
// old logs keep their text.
func TestRunOfKeepsRecordedIDs(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ id, session string }{
		{"human", "human"},
		{"claude:abc/x1", "claude:abc"},
		{"odd/sub", "odd"},
		{"claude:a:b/c/d", "claude:a:b"},
		{"", ""},
	} {
		r := RunOf(tc.id)
		if r.String() != tc.id || r.Session().String() != tc.session || SessionOf(tc.id) != tc.session {
			t.Errorf("%q: string %q session %q", tc.id, r, r.Session())
		}
	}
	if RunOf("human").Agent() != "" || RunOf("human").Short() != "human" {
		t.Error("human run")
	}
}

// runIDCut matches cutting or splitting a run-like value at the run-id
// separators.
var runIDCut = regexp.MustCompile(`strings\.(Cut|Split|SplitN|SplitAfter)\([^,]*(?i:run|session|actor|\bid\b)[^,]*, "[:/]"`)

// Only RunID cuts run ids apart (FH-91).
func TestNothingElseCutsRunIDs(t *testing.T) {
	t.Parallel()

	base := filepath.Join("..", "..")
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(base, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || filepath.Base(path) == "runid.go" {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for number, line := range strings.Split(string(data), "\n") {
				if runIDCut.MatchString(line) {
					t.Errorf("%s:%d cuts a run id: %s", path, number+1, strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
