package hooks

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// claudeToolName matches a quoted Claude Code tool name.
var claudeToolName = regexp.MustCompile(`"(Bash|PowerShell|Edit|Write|MultiEdit|NotebookEdit|TodoWrite|TaskCreate|TaskUpdate)"`)

// Only the Claude adapter knows Claude Code's tool names; past it, tool
// results are classified (FH-90). The one exception is runs' fallback for
// tool.used records from before protocol 2.
func TestNoClaudeToolNamesPastTheAdapter(t *testing.T) {
	t.Parallel()

	base := filepath.Join("..", "..")
	allowed := map[string]string{filepath.Join("internal", "runs", "runs.go"): "var legacyShellTools = "}
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(base, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			relative, _ := filepath.Rel(base, path)
			if entry.IsDir() {
				if relative == filepath.Join("internal", "hooks", "claude") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for number, line := range strings.Split(string(data), "\n") {
				if claudeToolName.MatchString(line) && !(allowed[relative] != "" && strings.HasPrefix(line, allowed[relative])) {
					t.Errorf("%s:%d names a Claude Code tool: %s", relative, number+1, strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
