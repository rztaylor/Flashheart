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
	// ErrBrokenFrontmatter means the frontmatter does not parse, so it cannot
	// be edited safely; the file needs repair by hand (STO-4).
	ErrBrokenFrontmatter = errors.New("frontmatter does not parse")
	// ErrNotFound means the section or checkbox does not exist.
	ErrNotFound = errors.New("not found")
)

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
	doc := Parse(data)
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

// SetList sets a top-level frontmatter field to a list: a flow list ([a, b])
// unless the field is already a block list, whose style is kept.
func SetList(data []byte, key string, items []string) ([]byte, error) {
	doc := Parse(data)
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
		return nil, fmt.Errorf("%w: %v", ErrBrokenFrontmatter, doc.FrontmatterError)
	}
	f := split(data)
	if !doc.HasFrontmatter {
		f.lines = append([]string{"---\n", replacement, "---\n"}, f.lines...)
		return f.bytes(), nil
	}
	closing := doc.BodyLine - 2 // index of the closing --- line
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
// section at the end of the file when it is missing.
func ReplaceSection(data []byte, sectionName, content string) ([]byte, error) {
	f := split(data)
	content = strings.Trim(strings.ReplaceAll(content, "\r\n", "\n"), "\n") + "\n"
	start, end, ok := f.section(sectionName)
	if !ok {
		f.lines = appendSection(f.lines, sectionName, content)
		return f.bytes(), nil
	}
	block := []string{"\n"}
	block = append(block, strings.SplitAfter(strings.TrimSuffix(content, "\n"), "\n")...)
	block[len(block)-1] += "\n"
	if end < len(f.lines) {
		block = append(block, "\n")
	}
	f.lines = slices.Concat(f.lines[:start+1], block, f.lines[end:])
	return f.bytes(), nil
}

// AppendToSection adds a paragraph to the end of a level-2 section (for
// example `## Notes`), replacing a lone "None." placeholder and adding the
// section when it is missing.
func AppendToSection(data []byte, sectionName, text string) ([]byte, error) {
	f := split(data)
	text = strings.Trim(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	start, end, ok := f.section(sectionName)
	if !ok {
		f.lines = appendSection(f.lines, sectionName, text+"\n")
		return f.bytes(), nil
	}
	existing := strings.TrimSpace(strings.Join(f.lines[start+1:end], ""))
	if existing == "" || placeholder(existing) {
		return ReplaceSection(data, sectionName, text)
	}
	return ReplaceSection(data, sectionName, existing+"\n\n"+text)
}

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
	lines = append(lines, "## "+name+"\n", "\n")
	for _, line := range strings.SplitAfter(strings.TrimSuffix(content, "\n"), "\n") {
		lines = append(lines, line)
	}
	lines[len(lines)-1] += "\n"
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
