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
