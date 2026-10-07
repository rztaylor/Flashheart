package board

import (
	"regexp"
	"strings"
)

// localLink matches a markdown link or image whose target is an absolute
// local path or a file:// URL, bare or in angle brackets (for paths with
// spaces): [text](/abs/path.png "title"), ![shot](</abs/my shot.png>).
var localLink = regexp.MustCompile(`(!?\[[^\]\n]*\]\()(?:<((?:file://)?/[^>\n]+)>|((?:file://)?/[^)\s]+))((?:\s+"[^"\n]*")?\))`)

// RewriteLocalLinks calls target with the path and text of each link or
// image in markdown to an absolute local path, outside code fences (so text
// can show example markdown), and points the link at what target returns;
// "" leaves it as it is (REV-5).
func RewriteLocalLinks(markdown string, target func(path, text string) string) string {
	lines := strings.Split(markdown, "\n")
	fenced := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		lines[index] = localLink.ReplaceAllStringFunc(line, func(match string) string {
			parts := localLink.FindStringSubmatch(match)
			text := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(parts[1], "!"), "["), "](")
			if to := target(strings.TrimPrefix(parts[2]+parts[3], "file://"), text); to != "" {
				return parts[1] + to + parts[4]
			}
			return match
		})
	}
	return strings.Join(lines, "\n")
}

// LocalLinks lists the absolute local paths markdown links to, outside
// code fences, in order.
func LocalLinks(markdown string) []string {
	var paths []string
	RewriteLocalLinks(markdown, func(path, _ string) string {
		paths = append(paths, path)
		return ""
	})
	return paths
}
