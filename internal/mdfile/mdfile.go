package mdfile

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Document is a parsed markdown file. Body and FrontmatterRaw use "\n" line
// endings regardless of the input.
type Document struct {
	HasFrontmatter   bool
	FrontmatterRaw   string
	FrontmatterError error
	Body             string
	// BodyLine is the 1-based line number in the file where Body starts.
	BodyLine int

	mapping *yaml.Node
	// plain is a whole-file YAML mapping (ParseYAML), not frontmatter.
	plain bool
	// insert is the 0-based file line a new field is inserted before: the
	// closing --- of frontmatter, or the end of a plain YAML document.
	insert int
}

// Field is one top-level frontmatter key in file order.
type Field struct {
	Key  string
	Line int
	node *yaml.Node
}

// Parse splits data into frontmatter and body. It never fails; problems are
// reported in FrontmatterError.
func Parse(data []byte) Document {
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.SplitAfter(text, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], " \t\n") != "---" {
		return Document{Body: text, BodyLine: 1}
	}

	closing := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimRight(lines[index], " \t\n") == "---" {
			closing = index
			break
		}
	}
	if closing < 0 {
		return Document{
			HasFrontmatter:   true,
			FrontmatterError: errors.New("frontmatter is not closed: add a line containing only ---"),
			Body:             strings.Join(lines[1:], ""),
			BodyLine:         2,
		}
	}

	doc := Document{
		HasFrontmatter: true,
		FrontmatterRaw: strings.Join(lines[1:closing], ""),
		Body:           strings.Join(lines[closing+1:], ""),
		BodyLine:       closing + 2,
		insert:         closing,
	}
	mapping, err := parseMapping(doc.FrontmatterRaw, 1)
	switch {
	case err != nil && mapping != nil:
		doc.FrontmatterError = fmt.Errorf("line %d: frontmatter must be a mapping of fields", mapping.Line+1)
	case err != nil:
		doc.FrontmatterError = errors.New("frontmatter does not parse: " + err.Error())
	default:
		doc.mapping = mapping
	}
	return doc
}

// ParseYAML parses a plain YAML file whose top level is a mapping of
// fields, such as project.yaml, ignoring a byte-order mark and Windows line
// endings. It never fails: a file that does not parse, or is not a
// mapping, has no fields and reports why in FrontmatterError. String, List,
// Decode and Fields read it as they read frontmatter, with file line
// numbers; it has no body.
func ParseYAML(data []byte) Document {
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	doc := Document{plain: true}
	mapping, err := parseMapping(text, 0)
	switch {
	case err != nil && mapping != nil:
		doc.FrontmatterError = fmt.Errorf("line %d: the file must be a mapping of fields", mapping.Line)
	case err != nil:
		doc.FrontmatterError = err
	default:
		doc.mapping = mapping
	}
	// New fields go at the end of the first YAML document: before a
	// "---" or "..." marker that follows its start, or at the end of file.
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	start := 0
	if mapping != nil {
		start = mapping.Line // the line after the first key
	} else {
		for start < len(lines) && isGap(lines[start]) {
			start++
		}
		if start < len(lines) && strings.TrimRight(lines[start], " \t\n") == "---" {
			start++
		}
	}
	doc.insert = len(lines)
	for index := start; index < len(lines); index++ {
		if marker := strings.TrimRight(lines[index], " \t\n"); marker == "---" || marker == "..." {
			doc.insert = index
			break
		}
	}
	return doc
}

// parseMapping parses YAML text whose first line is file line offset+1.
// An empty document is a nil mapping and no error; a document that is not
// a mapping returns its root node with an error.
func parseMapping(text string, offset int) (*yaml.Node, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(text), &root); err != nil {
		return nil, lineError(err, offset)
	}
	if len(root.Content) == 0 {
		return nil, nil
	}
	if mapping := root.Content[0]; mapping.Kind != yaml.MappingNode {
		return mapping, errors.New("not a mapping")
	}
	return root.Content[0], nil
}

var yamlLine = regexp.MustCompile(`line (\d+):`)

// lineError rewrites a YAML error's line numbers to file line numbers (YAML
// line 1 is file line offset+1) and drops its "yaml:" prefix.
func lineError(err error, offset int) error {
	message := err.Error()
	var typeError *yaml.TypeError
	if errors.As(err, &typeError) {
		message = strings.Join(typeError.Errors, "; ")
	}
	message = strings.TrimPrefix(message, "yaml: ")
	message = yamlLine.ReplaceAllStringFunc(message, func(match string) string {
		var line int
		fmt.Sscanf(match, "line %d:", &line)
		return fmt.Sprintf("line %d:", line+offset)
	})
	return errors.New(message)
}

// Fields returns the top-level frontmatter keys in file order, or nil when
// there is no parseable frontmatter.
func (d Document) Fields() []Field {
	if d.mapping == nil {
		return nil
	}
	fields := make([]Field, 0, len(d.mapping.Content)/2)
	for index := 0; index+1 < len(d.mapping.Content); index += 2 {
		key := d.mapping.Content[index]
		fields = append(fields, Field{Key: key.Value, Line: key.Line + d.offset(), node: d.mapping.Content[index+1]})
	}
	return fields
}

// offset is the file line before the YAML's first line.
func (d Document) offset() int {
	if d.plain {
		return 0
	}
	return 1
}

// Decode decodes a top-level field into v by YAML's typing rules, as
// yaml.Unmarshal would. A missing field leaves v unchanged; a field of the
// wrong type returns an error naming its file line.
func (d Document) Decode(key string, v any) error {
	node, ok := d.field(key)
	if !ok {
		return nil
	}
	if err := node.Decode(v); err != nil {
		return lineError(err, d.offset())
	}
	return nil
}

func (d Document) field(key string) (*yaml.Node, bool) {
	for _, field := range d.Fields() {
		if field.Key == key {
			return field.node, true
		}
	}
	return nil, false
}

// String returns a scalar field's value. A present but empty or null field
// returns "" and true; a non-scalar returns "" and true.
func (d Document) String(key string) (string, bool) {
	node, ok := d.field(key)
	if !ok {
		return "", false
	}
	if node.Kind == yaml.ScalarNode && node.Tag != "!!null" {
		return strings.TrimSpace(node.Value), true
	}
	return "", true
}

// List returns a field as a list of strings: a sequence's scalar items, or a
// non-empty scalar as a one-item list. Missing or empty fields return nil.
func (d Document) List(key string) []string {
	node, ok := d.field(key)
	if !ok {
		return nil
	}
	switch node.Kind {
	case yaml.SequenceNode:
		var items []string
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode && strings.TrimSpace(item.Value) != "" && item.Tag != "!!null" {
				items = append(items, strings.TrimSpace(item.Value))
			}
		}
		return items
	case yaml.ScalarNode:
		if value := strings.TrimSpace(node.Value); value != "" && node.Tag != "!!null" {
			return []string{value}
		}
	}
	return nil
}

// Section is a level-2 heading and the content up to the next heading of
// level 1 or 2.
type Section struct {
	Heading string
	Content string
	// Line is the 1-based line of the heading within the body.
	Line int
}

var heading = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*?)[ \t]*#*[ \t]*$`)

type bodyLine struct {
	text    string
	inFence bool
}

// scan marks the lines of body that are inside fenced code blocks.
func scan(body string) []bodyLine {
	raw := strings.SplitAfter(body, "\n")
	lines := make([]bodyLine, 0, len(raw))
	fence := ""
	for _, line := range raw {
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		marker := ""
		if indent <= 3 && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) {
			marker = trimmed[:3]
		}
		switch {
		case fence == "" && marker != "":
			fence = marker
			lines = append(lines, bodyLine{text: line, inFence: true})
		case fence != "" && marker == fence:
			fence = ""
			lines = append(lines, bodyLine{text: line, inFence: true})
		default:
			lines = append(lines, bodyLine{text: line, inFence: fence != ""})
		}
	}
	return lines
}

// Title returns the text of the first level-1 heading outside code fences.
func Title(body string) string {
	for _, line := range scan(body) {
		if line.inFence {
			continue
		}
		if match := heading.FindStringSubmatch(strings.TrimRight(line.text, "\n")); match != nil && len(match[1]) == 1 {
			return match[2]
		}
	}
	return ""
}

// Sections returns the level-2 sections of body in order.
func Sections(body string) []Section {
	var sections []Section
	var current *Section
	var content strings.Builder
	flush := func() {
		if current != nil {
			current.Content = content.String()
			sections = append(sections, *current)
			current = nil
		}
		content.Reset()
	}
	for index, line := range scan(body) {
		if !line.inFence {
			if match := heading.FindStringSubmatch(strings.TrimRight(line.text, "\n")); match != nil && len(match[1]) <= 2 {
				flush()
				if len(match[1]) == 2 {
					current = &Section{Heading: match[2], Line: index + 1}
				}
				continue
			}
		}
		if current != nil {
			content.WriteString(line.text)
		}
	}
	flush()
	return sections
}

// FindSection returns the first level-2 section whose heading matches name,
// ignoring case.
func FindSection(body, name string) (Section, bool) {
	for _, section := range Sections(body) {
		if strings.EqualFold(section.Heading, name) {
			return section, true
		}
	}
	return Section{}, false
}

// Checkbox is a top-level task-list item.
type Checkbox struct {
	Text    string
	Checked bool
}

var taskItem = regexp.MustCompile(`^[-*+] \[([ xX])\] (.*?)\s*$`)

// Checkboxes returns the unindented task-list items in content, outside code
// fences.
func Checkboxes(content string) []Checkbox {
	var boxes []Checkbox
	for _, line := range scan(content) {
		if line.inFence {
			continue
		}
		if match := taskItem.FindStringSubmatch(strings.TrimRight(line.text, "\n")); match != nil {
			boxes = append(boxes, Checkbox{Text: match[2], Checked: match[1] != " "})
		}
	}
	return boxes
}
