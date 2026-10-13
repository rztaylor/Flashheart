package setup

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Codex's config.toml is edited as text, not decoded and re-encoded: Codex
// writes the file itself (project and hook trust) and the user's comments,
// order and spacing must survive. scanTOML finds where each table header
// and key/value statement is, which is all setup needs to replace or remove
// its own table; values are read only for the few keys setup checks.

// tomlEntry is one statement of a TOML document.
type tomlEntry struct {
	// start and end span the statement's lines: from the start of its
	// first line to after the newline that ends it (or the end of input).
	start, end int
	// header is a [table] or, with array, an [[array of tables]] header.
	header, array bool
	// path is a header's table, or a key's full path (its table, then its
	// dotted key), unquoted.
	path []string
	// value is a key's raw value text, without a trailing comment.
	value string
}

// scanTOML splits data into statements. It understands TOML's syntax well
// enough to find statements (strings of every kind, comments, arrays and
// inline tables over several lines) but does not validate values.
func scanTOML(data []byte) ([]tomlEntry, error) {
	text := string(data)
	var entries []tomlEntry
	var table []string
	pos := 0
	fail := func(at int, message string) error {
		return fmt.Errorf("line %d: %s", strings.Count(text[:min(at, len(text))], "\n")+1, message)
	}
	for pos < len(text) {
		start := pos
		at := skipBlank(text, pos)
		if at >= len(text) {
			break
		}
		switch text[at] {
		case '\n':
			pos = at + 1
			continue
		case '\r':
			pos = at + 1
			continue
		case '#':
			pos = lineEnd(text, at)
			continue
		}
		entry := tomlEntry{start: start}
		var err error
		if text[at] == '[' {
			entry.header = true
			at++
			if at < len(text) && text[at] == '[' {
				entry.array = true
				at++
			}
			if entry.path, at, err = scanKey(text, at); err != nil {
				return nil, fail(at, err.Error())
			}
			at = skipBlank(text, at)
			closing := "]"
			if entry.array {
				closing = "]]"
			}
			if !strings.HasPrefix(text[at:], closing) {
				return nil, fail(at, "a table header is not closed")
			}
			at += len(closing)
			table = entry.path
		} else {
			var key []string
			if key, at, err = scanKey(text, at); err != nil {
				return nil, fail(at, err.Error())
			}
			at = skipBlank(text, at)
			if at >= len(text) || text[at] != '=' {
				return nil, fail(at, "a key has no = and value")
			}
			valueStart := skipBlank(text, at+1)
			if at, err = scanValue(text, valueStart); err != nil {
				return nil, fail(valueStart, err.Error())
			}
			entry.value = strings.TrimRight(text[valueStart:at], " \t\r")
			if entry.value == "" {
				return nil, fail(valueStart, "a key has no value")
			}
			entry.path = append(append([]string{}, table...), key...)
		}
		// Only a comment may follow on the statement's last line.
		at = skipBlank(text, at)
		if at < len(text) && text[at] != '#' && text[at] != '\n' && !strings.HasPrefix(text[at:], "\r\n") {
			return nil, fail(at, fmt.Sprintf("unexpected %q after a statement", text[at]))
		}
		entry.end = lineEnd(text, at)
		pos = entry.end
		entries = append(entries, entry)
	}
	return entries, nil
}

func skipBlank(text string, at int) int {
	for at < len(text) && (text[at] == ' ' || text[at] == '\t') {
		at++
	}
	return at
}

// lineEnd is the index after the newline ending at's line, or len(text).
func lineEnd(text string, at int) int {
	if index := strings.IndexByte(text[at:], '\n'); index >= 0 {
		return at + index + 1
	}
	return len(text)
}

// scanKey reads a dotted key of bare and quoted parts.
func scanKey(text string, at int) ([]string, int, error) {
	var path []string
	for {
		at = skipBlank(text, at)
		if at >= len(text) {
			return nil, at, errors.New("a key is cut short")
		}
		switch text[at] {
		case '"', '\'':
			end, err := stringEnd(text, at)
			if err != nil {
				return nil, at, err
			}
			part, ok := parseTOMLString(text[at:end])
			if !ok {
				return nil, at, errors.New("a quoted key cannot be read")
			}
			path = append(path, part)
			at = end
		default:
			end := at
			for end < len(text) && isBareKey(text[end]) {
				end++
			}
			if end == at {
				return nil, at, fmt.Errorf("unexpected %q where a key belongs", text[at])
			}
			path = append(path, text[at:end])
			at = end
		}
		at = skipBlank(text, at)
		if at >= len(text) || text[at] != '.' {
			return path, at, nil
		}
		at++
	}
}

func isBareKey(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// scanValue returns where the value starting at at ends: at the comment or
// newline that follows it outside strings, arrays and inline tables.
func scanValue(text string, at int) (int, error) {
	depth := 0
	for at < len(text) {
		switch c := text[at]; c {
		case '"', '\'':
			end, err := stringEnd(text, at)
			if err != nil {
				return at, err
			}
			at = end
			continue
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth < 0 {
				return at, fmt.Errorf("unexpected %q", c)
			}
		case '#':
			if depth == 0 {
				return at, nil
			}
			at = lineEnd(text, at)
			continue
		case '\n':
			if depth == 0 {
				return at, nil
			}
		}
		at++
	}
	if depth > 0 {
		return at, errors.New("an array or inline table is not closed")
	}
	return at, nil
}

// stringEnd returns the index after the string starting at at: basic or
// literal, single-line or multi-line.
func stringEnd(text string, at int) (int, error) {
	quote := text[at]
	escapes := quote == '"'
	if triple := strings.Repeat(string(quote), 3); strings.HasPrefix(text[at:], triple) {
		for index := at + 3; index < len(text); index++ {
			if escapes && text[index] == '\\' {
				index++
				continue
			}
			if strings.HasPrefix(text[index:], triple) {
				end := index + 3
				// Up to two quotes may end the content itself.
				for extra := 0; extra < 2 && end < len(text) && text[end] == quote; extra++ {
					end++
				}
				return end, nil
			}
		}
		return at, errors.New("a multi-line string is not closed")
	}
	for index := at + 1; index < len(text); index++ {
		switch {
		case escapes && text[index] == '\\':
			index++
		case text[index] == quote:
			return index + 1, nil
		case text[index] == '\n':
			return at, errors.New("a string is not closed on its line")
		}
	}
	return at, errors.New("a string is not closed")
}

// parseTOMLString reads a single-line basic or literal string that is the
// whole of raw.
func parseTOMLString(raw string) (string, bool) {
	if len(raw) < 2 || strings.HasPrefix(raw, `"""`) || strings.HasPrefix(raw, "'''") {
		return "", false
	}
	end, err := stringEnd(raw, 0)
	if err != nil || end != len(raw) {
		return "", false
	}
	inner := raw[1 : len(raw)-1]
	if raw[0] == '\'' {
		return inner, true
	}
	var out strings.Builder
	for index := 0; index < len(inner); index++ {
		c := inner[index]
		if c != '\\' {
			out.WriteByte(c)
			continue
		}
		index++
		if index >= len(inner) {
			return "", false
		}
		switch inner[index] {
		case 'b':
			out.WriteByte('\b')
		case 't':
			out.WriteByte('\t')
		case 'n':
			out.WriteByte('\n')
		case 'f':
			out.WriteByte('\f')
		case 'r':
			out.WriteByte('\r')
		case 'e':
			out.WriteByte(0x1b)
		case '"', '\\':
			out.WriteByte(inner[index])
		case 'u', 'U':
			digits := 4
			if inner[index] == 'U' {
				digits = 8
			}
			if index+1+digits > len(inner) {
				return "", false
			}
			code, err := strconv.ParseUint(inner[index+1:index+1+digits], 16, 32)
			if err != nil || !utf8.ValidRune(rune(code)) {
				return "", false
			}
			out.WriteRune(rune(code))
			index += digits
		default:
			return "", false
		}
	}
	return out.String(), true
}

// parseTOMLStringArray reads an array of strings, over several lines and
// with comments if need be.
func parseTOMLStringArray(raw string) ([]string, bool) {
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return nil, false
	}
	items := []string{}
	inner := raw[1 : len(raw)-1]
	expectItem := true
	for at := 0; at < len(inner); {
		switch c := inner[at]; {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			at++
		case c == '#':
			at = lineEnd(inner, at)
		case c == ',':
			if expectItem {
				return nil, false
			}
			expectItem = true
			at++
		case (c == '"' || c == '\'') && expectItem:
			end, err := stringEnd(inner, at)
			if err != nil {
				return nil, false
			}
			item, ok := parseTOMLString(inner[at:end])
			if !ok {
				return nil, false
			}
			items = append(items, item)
			expectItem = false
			at = end
		default:
			return nil, false
		}
	}
	return items, true
}

// tomlString writes s as a TOML basic string.
func tomlString(s string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			out.WriteByte('\\')
			out.WriteRune(r)
		case '\b':
			out.WriteString(`\b`)
		case '\t':
			out.WriteString(`\t`)
		case '\n':
			out.WriteString(`\n`)
		case '\f':
			out.WriteString(`\f`)
		case '\r':
			out.WriteString(`\r`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&out, `\u%04X`, r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}

// tomlStringArray writes items as a one-line TOML array of strings.
func tomlStringArray(items []string) string {
	quoted := make([]string, len(items))
	for index, item := range items {
		quoted[index] = tomlString(item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
