package protocol

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The marginal remarks (FH-22) are browser copy only: no Go code, and so no
// hook output, MCP result, recovery note or ticket write, carries one.
func TestNoMarginalRemarkReachesAgents(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "model", "remarks.md"))
	if err != nil {
		t.Fatal(err)
	}
	var remarks []string
	for _, match := range regexp.MustCompile(`(?m)^\d+\. (.+)$`).FindAllStringSubmatch(string(data), -1) {
		text := strings.Trim(match[1], "*“”")
		if before, _, ok := strings.Cut(text, "**"); ok {
			text = strings.Trim(before, "“”")
		}
		// Very short lines ("Woof!", "Wibble.") are too common to test for.
		if len(text) > 12 {
			remarks = append(remarks, text)
		}
	}
	if len(remarks) < 60 {
		t.Fatalf("read only %d remarks", len(remarks))
	}
	err = filepath.WalkDir(filepath.Join("..", ".."), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == "node_modules" || entry.Name() == ".cache" || entry.Name() == "frontend" || strings.HasPrefix(entry.Name(), ".")) && path != filepath.Join("..", "..") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, remark := range remarks {
			if strings.Contains(string(source), remark) {
				t.Errorf("%s carries the remark %q", path, remark)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
