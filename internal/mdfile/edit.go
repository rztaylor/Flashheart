package mdfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Edits return new file bytes and leave every line they do not own exactly as
// it was: other fields, comments, key order, unknown keys, line endings and a
// byte-order mark (STO-2). An edit that changes nothing returns data
// unchanged, so saving without changes is byte-identical.

var (
	// ErrBrokenFrontmatter means the frontmatter, or a plain YAML file, does
	// not parse, so it cannot be edited safely; the file needs repair by
	// hand (STO-4).
	ErrBrokenFrontmatter = errors.New("frontmatter does not parse")
	// ErrNotFound means the section or checkbox does not exist.
	ErrNotFound = errors.New("not found")
)

// brokenYAML is ErrBrokenFrontmatter for a plain YAML file.
type brokenYAML struct{ err error }

func (e brokenYAML) Error() string { return "the YAML does not parse: " + e.err.Error() }
func (e brokenYAML) Unwrap() error { return ErrBrokenFrontmatter }

// file is data split into editable lines with its original encoding.
type file struct {
	bom   bool
	crlf  bool
	lines []string // "\n"-terminated except possibly the last
}

func split(data []byte) file {
	f := file{bom: bytes.HasPrefix(data, []byte("\ufeff"))}
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	f.crlf = bytes.Contains(data, []byte("\r\n"))
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if text != "" {
		f.lines = strings.SplitAfter(text, "\n")
		if f.lines[len(f.lines)-1] == "" {
			f.lines = f.lines[:len(f.lines)-1]
		}
	}
	return f
}

func (f file) bytes() []byte {
	text := strings.Join(f.lines, "")
	if f.crlf {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	if f.bom {
		text = "\ufeff" + text
	}
	return []byte(text)
}

// SetScalar sets a top-level frontmatter field to a single string value,
// creating the frontmatter or field when missing. Values are written plain
// when that reads back identically, otherwise double-quoted.
func SetScalar(data []byte, key, value string) ([]byte, error) {
	return setScalar(data, Parse(data), key, value)
}

// SetList sets a top-level frontmatter field to a list: a flow list ([a, b])
// unless the field is already a block list, whose style is kept.
func SetList(data []byte, key string, items []string) ([]byte, error) {
	return setList(data, Parse(data), key, items)
}

// SetYAMLScalar is SetScalar for a plain YAML file whose top level is a
// mapping, such as project.yaml (see ParseYAML): a missing field is added
// at the end of the mapping. A file that does not parse is refused with
// ErrBrokenFrontmatter.
func SetYAMLScalar(data []byte, key, value string) ([]byte, error) {
	return setScalar(data, ParseYAML(data), key, value)
}

// SetYAMLList is SetList for a plain YAML file, as SetYAMLScalar.
func SetYAMLList(data []byte, key string, items []string) ([]byte, error) {
	return setList(data, ParseYAML(data), key, items)
}

func setScalar(data []byte, doc Document, key, value string) ([]byte, error) {
	if node, ok := doc.field(key); ok && node.Kind == yaml.ScalarNode {
		if current, _ := doc.String(key); current == value {
			return data, nil
		}
	}
	line := key + ":"
	if value != "" {
		line += " " + scalar(value)
	}
	return setField(data, doc, key, line+"\n")
}

func setList(data []byte, doc Document, key string, items []string) ([]byte, error) {
	// A scalar holding the same single item (tags: ui) already reads the same.
	if _, ok := doc.field(key); ok && slices.Equal(doc.List(key), items) {
		return data, nil
	}
	quoted := make([]string, len(items))
	for index, item := range items {
		quoted[index] = scalar(item)
	}
	// A list already written in block style keeps that style and indent.
	if node, ok := doc.field(key); ok && node.Kind == yaml.SequenceNode && node.Style&yaml.FlowStyle == 0 &&
		len(node.Content) > 0 && len(items) > 0 {
		indent := strings.Repeat(" ", max(node.Content[0].Column-3, 0))
		var block strings.Builder
		block.WriteString(key + ":\n")
		for _, item := range quoted {
			block.WriteString(indent + "- " + item + "\n")
		}
		return setField(data, doc, key, block.String())
	}
	return setField(data, doc, key, key+": ["+strings.Join(quoted, ", ")+"]\n")
}

func setField(data []byte, doc Document, key, replacement string) ([]byte, error) {
	if doc.FrontmatterError != nil {
		if doc.plain {
			return nil, brokenYAML{doc.FrontmatterError}
		}
		return nil, fmt.Errorf("%w: %v", ErrBrokenFrontmatter, doc.FrontmatterError)
	}
	f := split(data)
	if !doc.HasFrontmatter && !doc.plain {
		f.lines = append([]string{"---\n", replacement, "---\n"}, f.lines...)
		return f.bytes(), nil
	}
	closing := doc.insert // the closing --- line, or the end of the YAML
	fields := doc.Fields()
	for index, field := range fields {
		if field.Key != key {
			continue
		}
		start := field.Line - 1
		end := closing
		if index+1 < len(fields) {
			end = fields[index+1].Line - 1
		}
		// Comments and blank lines before the next key stay where they are.
		for end > start+1 && isGap(f.lines[end-1]) {
			end--
		}
		f.lines = slices.Concat(f.lines[:start], []string{replacement}, f.lines[end:])
		return f.bytes(), nil
	}
	// A field added after a plain file's unterminated last line ends it.
	if closing > 0 && closing == len(f.lines) && !strings.HasSuffix(f.lines[closing-1], "\n") {
		f.lines[closing-1] += "\n"
	}
	f.lines = slices.Insert(f.lines, closing, replacement)
	return f.bytes(), nil
}

func isGap(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || (strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(line, " "))
}

var (
	plain    = regexp.MustCompile(`^[A-Za-z0-9_./()][A-Za-z0-9 _./()+:@'-]*$`)
	keywords = map[string]bool{"null": true, "~": true, "true": true, "false": true, "yes": true, "no": true, "on": true, "off": true}
)

// scalar renders value as YAML: plain when that reads back as the same
// string, otherwise as a double-quoted (JSON) string. Plain values never
// contain , [ ] { } so they are also safe inside flow lists.
func scalar(value string) string {
	if plain.MatchString(value) && !strings.Contains(value, ": ") && !strings.HasSuffix(value, ":") &&
		!strings.HasSuffix(value, " ") && !keywords[strings.ToLower(value)] {
		var parsed map[string]string
		if yaml.Unmarshal([]byte("k: "+value), &parsed) == nil && parsed["k"] == value {
			return value
		}
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	return strings.TrimSuffix(out.String(), "\n")
}

// section locates a level-2 section in f: the heading line index and the
// index one past its last line.
func (f file) section(name string) (int, int, bool) {
	body, offset := f.body()
	lines := scan(strings.Join(body, ""))
	start := -1
	for index, line := range lines {
		if line.inFence {
			continue
		}
		match := heading.FindStringSubmatch(strings.TrimRight(line.text, "\n"))
		if match == nil || len(match[1]) > 2 {
			continue
		}
		if start >= 0 {
			return offset + start, offset + index, true
		}
		if len(match[1]) == 2 && strings.EqualFold(match[2], name) {
			start = index
		}
	}
	if start >= 0 {
		// scan ends with an empty line after a final newline; stop at the file.
		return offset + start, len(f.lines), true
	}
	return 0, 0, false
}

// body returns the lines after the frontmatter and their index in f.
func (f file) body() ([]string, int) {
	doc := Parse([]byte(strings.Join(f.lines, "")))
	offset := doc.BodyLine - 1
	if offset > len(f.lines) {
		offset = len(f.lines)
	}
	return f.lines[offset:], offset
}

// SetCheckbox ticks or unticks the index-th top-level task-list item in the
// named level-2 section (CARD-3).
func SetCheckbox(data []byte, sectionName string, index int, checked bool) ([]byte, error) {
	f := split(data)
	start, end, ok := f.section(sectionName)
	if !ok {
		return nil, fmt.Errorf("section %q: %w", sectionName, ErrNotFound)
	}
	seen := 0
	for position, line := range scan(strings.Join(f.lines[start:end], "")) {
		if line.inFence || !taskItem.MatchString(strings.TrimRight(line.text, "\n")) {
			continue
		}
		if seen == index {
			mark := " "
			if checked {
				mark = "x"
			}
			text := f.lines[start+position]
			updated := text[:3] + mark + text[4:]
			if updated == text || (checked && text[3] != ' ') {
				return data, nil
			}
			f.lines[start+position] = updated
			return f.bytes(), nil
		}
		seen++
	}
	return nil, fmt.Errorf("checkbox %d in %q: %w", index, sectionName, ErrNotFound)
}

// ReplaceSection replaces the content of a level-2 section, adding the
// section at the end of the file when it is missing. The content stays
// inside the section (SectionSafe). Content equal to the section's own,
// ignoring blank lines around it and trailing whitespace, changes nothing
// (STO-2); empty content leaves the heading and a blank line before the
// next one, as the template writes an empty section.
func ReplaceSection(data []byte, sectionName, content string) ([]byte, error) {
	return PutSection(data, sectionName, content)
}

// PutSection is ReplaceSection, except that a missing section is added
// before the earliest section in the file that before names (outside code
// fences), and at the end only when the file has none of them: a ticket's
// Context goes above its Notes and Handoff (FH-66).
func PutSection(data []byte, sectionName, content string, before ...string) ([]byte, error) {
	f := split(data)
	content = SectionSafe(sectionText(strings.ReplaceAll(content, "\r\n", "\n")))
	start, end, ok := f.section(sectionName)
	if !ok {
		at := len(f.lines)
		for _, name := range before {
			if next, _, found := f.section(name); found && next < at {
				at = next
			}
		}
		if at == len(f.lines) {
			f.lines = appendSection(f.lines, sectionName, content)
			return f.bytes(), nil
		}
		block := []string{"## " + sectionName + "\n", "\n"}
		if at > 0 && strings.TrimSpace(f.lines[at-1]) != "" {
			block = append([]string{"\n"}, block...)
		}
		if content != "" {
			block = append(append(block, contentLines(content)...), "\n")
		}
		f.lines = slices.Concat(f.lines[:at], block, f.lines[at:])
		return f.bytes(), nil
	}
	if sectionText(strings.Join(f.lines[start+1:end], "")) == content {
		return data, nil
	}
	var block []string
	if content != "" {
		block = append([]string{"\n"}, contentLines(content)...)
	}
	if end < len(f.lines) {
		block = append(block, "\n")
	}
	if len(block) > 0 && !strings.HasSuffix(f.lines[start], "\n") {
		f.lines[start] += "\n"
	}
	f.lines = slices.Concat(f.lines[:start+1], block, f.lines[end:])
	return f.bytes(), nil
}

// sectionText is a section's text as it is compared and written: without
// the blank lines before it or the whitespace after it.
func sectionText(text string) string {
	text = strings.TrimRight(text, " \t\r\n")
	for {
		line, rest, found := strings.Cut(text, "\n")
		if !found || strings.TrimSpace(line) != "" {
			return text
		}
		text = rest
	}
}

// contentLines splits text that has no final newline into lines that end
// in one.
func contentLines(text string) []string {
	lines := strings.SplitAfter(text, "\n")
	lines[len(lines)-1] += "\n"
	return lines
}

// AppendToSection adds a paragraph to the end of a level-2 section (for
// example `## Notes`), replacing a lone "None." placeholder and adding the
// section when it is missing.
func AppendToSection(data []byte, sectionName, text string) ([]byte, error) {
	f := split(data)
	text = SectionSafe(sectionText(strings.ReplaceAll(text, "\r\n", "\n")))
	start, end, ok := f.section(sectionName)
	if !ok {
		f.lines = appendSection(f.lines, sectionName, text)
		return f.bytes(), nil
	}
	existing := strings.TrimSpace(strings.Join(f.lines[start+1:end], ""))
	if existing == "" || placeholder(existing) {
		return ReplaceSection(data, sectionName, text)
	}
	return ReplaceSection(data, sectionName, existing+"\n\n"+text)
}

// SectionSafe keeps written content inside its section: a level-1 or -2
// heading outside code fences, which would end the section, becomes a
// level-3 one, and a code fence left open, which would hide the headings
// after it, is closed.
func SectionSafe(content string) string { return HeadingsFrom(content, 3) }

// HeadingsFrom keeps written content inside a part of a file that headings
// above level would end: each such heading outside code fences becomes a
// heading of level, and a code fence left open is closed. SectionSafe is
// HeadingsFrom at level 3; an activity entry's text uses level 4, since its
// entries start at level-3 headings.
func HeadingsFrom(content string, level int) string {
	var out strings.Builder
	for _, line := range scan(content) {
		text := line.text
		if !line.inFence {
			if match := heading.FindStringSubmatch(strings.TrimRight(text, "\n")); match != nil && len(match[1]) < level {
				text = strings.Repeat("#", level-len(match[1])) + strings.TrimLeft(text, " ")
			}
		}
		out.WriteString(text)
	}
	if marker := openFence(content); marker != "" {
		out.WriteString("\n" + marker)
	}
	return out.String()
}

// openFence is the marker of a code fence content leaves open, or "".
func openFence(content string) string {
	fence := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if len(line)-len(trimmed) > 3 || len(trimmed) < 3 {
			continue
		}
		switch marker := trimmed[:3]; {
		case marker != "```" && marker != "~~~":
		case fence == "":
			fence = marker
		case fence == marker:
			fence = ""
		}
	}
	return fence
}

// IsPlaceholder reports text that only stands for an empty section, such
// as "None." or "TBD", which the first real content replaces.
func IsPlaceholder(text string) bool { return placeholder(strings.TrimSpace(text)) }

func placeholder(text string) bool {
	switch strings.ToLower(strings.Trim(text, "_* .")) {
	case "none", "n/a", "tbd":
		return true
	}
	return false
}

func appendSection(lines []string, name, content string) []string {
	if len(lines) > 0 && !strings.HasSuffix(lines[len(lines)-1], "\n") {
		lines[len(lines)-1] += "\n"
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "\n")
	}
	lines = append(lines, "## "+name+"\n")
	if content != "" {
		lines = append(append(lines, "\n"), contentLines(content)...)
	}
	return lines
}

// SetTitle replaces the first level-1 heading outside code fences, or adds
// one at the top of the body.
func SetTitle(data []byte, title string) ([]byte, error) {
	title = strings.Join(strings.Fields(title), " ")
	f := split(data)
	body, offset := f.body()
	for index, line := range scan(strings.Join(body, "")) {
		if line.inFence {
			continue
		}
		match := heading.FindStringSubmatch(strings.TrimRight(line.text, "\n"))
		if match == nil || len(match[1]) != 1 {
			continue
		}
		if match[2] == title {
			return data, nil
		}
		f.lines[offset+index] = "# " + title + "\n"
		return f.bytes(), nil
	}
	rest := slices.Clone(f.lines[offset:])
	for len(rest) > 0 && strings.TrimSpace(rest[0]) == "" {
		rest = rest[1:]
	}
	top := []string{"# " + title + "\n"}
	if len(rest) > 0 {
		top = append(top, "\n")
	}
	f.lines = slices.Concat(f.lines[:offset], top, rest)
	return f.bytes(), nil
}
