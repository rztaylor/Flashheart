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
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(doc.FrontmatterRaw), &root); err != nil {
		doc.FrontmatterError = frontmatterError(err)
		return doc
	}
	if len(root.Content) == 0 {
		return doc
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		doc.FrontmatterError = fmt.Errorf("line %d: frontmatter must be a mapping of fields", mapping.Line+1)
		return doc
	}
	doc.mapping = mapping
	return doc
}

var yamlLine = regexp.MustCompile(`line (\d+):`)

// frontmatterError rewrites YAML line numbers to file line numbers (the
// frontmatter starts on line 2).
func frontmatterError(err error) error {
	message := strings.TrimPrefix(err.Error(), "yaml: ")
	message = yamlLine.ReplaceAllStringFunc(message, func(match string) string {
		var line int
		fmt.Sscanf(match, "line %d:", &line)
		return fmt.Sprintf("line %d:", line+1)
	})
	return errors.New("frontmatter does not parse: " + message)
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
		fields = append(fields, Field{Key: key.Value, Line: key.Line + 1, node: d.mapping.Content[index+1]})
	}
	return fields
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
