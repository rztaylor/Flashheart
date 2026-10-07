package board

import (
	"strings"
	"testing"
)

func TestRewriteLocalLinks(t *testing.T) {
	t.Parallel()

	markdown := strings.Join([]string{
		`![Board](</Users/a/my shot.png>) and [log](file:///tmp/run.log "title")`,
		"```",
		"![example](/tmp/example.png)",
		"```",
		"[web](https://example.com/a.png) [relative](files/a.png)",
	}, "\n")
	var seen []string
	got := RewriteLocalLinks(markdown, func(path, text string) string {
		seen = append(seen, text+"="+path)
		if strings.HasSuffix(path, ".png") {
			return "files/copy.png"
		}
		return ""
	})
	if want := "Board=/Users/a/my shot.png|log=/tmp/run.log"; strings.Join(seen, "|") != want {
		t.Fatalf("seen = %q", seen)
	}
	if !strings.HasPrefix(got, `![Board](files/copy.png) and [log](file:///tmp/run.log "title")`) || !strings.Contains(got, "![example](/tmp/example.png)") {
		t.Fatalf("got:\n%s", got)
	}
	if links := LocalLinks(markdown); len(links) != 2 {
		t.Fatalf("LocalLinks = %q", links)
	}
}

func TestRewriteLocalLinksLeavesInlineCodeAlone(t *testing.T) {
	t.Parallel()

	line := "Write `![x](/tmp/a.png)` or ``![y](/tmp/b.png) ` `` but ![z](/tmp/c.png)"
	got := RewriteLocalLinks(line, func(path, _ string) string { return "files/copy.png" })
	if want := "Write `![x](/tmp/a.png)` or ``![y](/tmp/b.png) ` `` but ![z](files/copy.png)"; got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	// An unclosed backtick is literal text, not code.
	if got := RewriteLocalLinks("a ` ![z](/tmp/c.png)", func(string, string) string { return "c" }); got != "a ` ![z](c)" {
		t.Fatalf("unclosed: %q", got)
	}
}
