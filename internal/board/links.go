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
		lines[index] = outsideCode(line, func(text string) string {
			return localLink.ReplaceAllStringFunc(text, func(match string) string {
				parts := localLink.FindStringSubmatch(match)
				label := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(parts[1], "!"), "["), "](")
				if to := target(strings.TrimPrefix(parts[2]+parts[3], "file://"), label); to != "" {
					return parts[1] + to + parts[4]
				}
				return match
			})
		})
	}
	return strings.Join(lines, "\n")
}

// outsideCode applies edit to the parts of line outside inline code spans:
// a run of backticks up to the next run of the same length (CommonMark).
func outsideCode(line string, edit func(string) string) string {
	var out strings.Builder
	plain := 0
	for at := 0; at < len(line); {
		if line[at] != '`' {
			at++
			continue
		}
		run := at
		for run < len(line) && line[run] == '`' {
			run++
		}
		ticks := line[at:run]
		end := -1
		for search := run; search < len(line); {
			next := strings.Index(line[search:], ticks)
			if next < 0 {
				break
			}
			next += search
			after := next + len(ticks)
			if (next == 0 || line[next-1] != '`') && (after == len(line) || line[after] != '`') {
				end = after
				break
			}
			search = after
			for search < len(line) && line[search] == '`' {
				search++
			}
		}
		if end < 0 {
			at = run
			continue
		}
		out.WriteString(edit(line[plain:at]))
		out.WriteString(line[at:end])
		plain, at = end, end
	}
	out.WriteString(edit(line[plain:]))
	return out.String()
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
