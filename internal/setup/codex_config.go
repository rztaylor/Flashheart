package setup

import (
	"errors"
	"slices"
	"sort"
	"strings"
)

// serverPath is the flashheart MCP server's table in Codex's config.toml.
var serverPath = []string{"mcp_servers", serverName}

// serverTable is where config.toml defines the flashheart MCP server.
type serverTable struct {
	// header is the [mcp_servers.flashheart] header, or nil.
	header *tomlEntry
	// command and args are its keys, or nil.
	command, args *tomlEntry
	// spans are the server's tables, its own and any below it such as
	// [mcp_servers.flashheart.env], each from its header to its last
	// statement.
	spans [][2]int
	// insertAt is where a new table goes: after the last MCP server's
	// table, else at the end.
	insertAt int
}

// findServer locates the flashheart MCP server in config.toml. A server
// defined any other way than as its own table (dotted keys, an inline
// table) is refused rather than guessed at.
func findServer(data []byte) (serverTable, error) {
	entries, err := scanTOML(data)
	if err != nil {
		return serverTable{}, err
	}
	s := serverTable{insertAt: len(data)}
	var current []string
	inServer := false
	for index := range entries {
		entry := &entries[index]
		if entry.header {
			current = entry.path
			inServer = hasPrefix(entry.path, serverPath)
			if inServer {
				if slices.Equal(entry.path, serverPath) {
					if entry.array || s.header != nil {
						return serverTable{}, errors.New("[mcp_servers.flashheart] is defined more than once")
					}
					s.header = entry
				}
				s.spans = append(s.spans, [2]int{entry.start, entry.end})
			}
		} else {
			switch {
			case inServer:
				s.spans[len(s.spans)-1][1] = entry.end
				if slices.Equal(current, serverPath) && len(entry.path) == len(serverPath)+1 {
					switch entry.path[len(serverPath)] {
					case "command":
						s.command = entry
					case "args":
						s.args = entry
					}
				}
			case hasPrefix(entry.path, serverPath):
				return serverTable{}, errors.New("mcp_servers.flashheart is set outside its own [mcp_servers.flashheart] table; move it into one, or remove it")
			case slices.Equal(entry.path, serverPath[:1]):
				return serverTable{}, errors.New("mcp_servers is an inline table, which setup cannot add to; write each server as its own [mcp_servers.<name>] table")
			}
		}
		if len(current) > 0 && current[0] == serverPath[0] {
			s.insertAt = entry.end
		}
	}
	return s, nil
}

func hasPrefix(path, prefix []string) bool {
	return len(path) >= len(prefix) && slices.Equal(path[:len(prefix)], prefix)
}

// installedConfig is config.toml with the flashheart MCP server running
// this binary: its command and args are set, everything else in the file,
// the rest of its table included, stays as written.
func (o Options) installedConfig(data []byte) ([]byte, error) {
	s, err := findServer(data)
	if err != nil {
		return nil, err
	}
	command := "command = " + tomlString(o.Binary) + "\n"
	args := "args = " + tomlStringArray(o.serverArgs()) + "\n"
	text := string(data)
	if s.header == nil {
		table := "[" + strings.Join(serverPath, ".") + "]\n" + command + args
		prefix := ""
		if text != "" {
			if s.insertAt == len(text) && !strings.HasSuffix(text, "\n") {
				prefix = "\n"
			}
			prefix += "\n"
		}
		return []byte(text[:s.insertAt] + prefix + table + text[s.insertAt:]), nil
	}
	// Each edit replaces text[start:end]; a missing key is inserted after
	// the header (args after command when command is there).
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	insert := func(at int, line string) {
		if !strings.HasSuffix(text[:at], "\n") {
			line = "\n" + line
		}
		edits = append(edits, edit{at, at, line})
	}
	replace := func(entry *tomlEntry, line string) {
		indent := text[entry.start:skipBlank(text, entry.start)]
		edits = append(edits, edit{entry.start, entry.end, indent + line})
	}
	configured, _ := parseTOMLString(entryValue(s.command))
	currentArgs, _ := parseTOMLStringArray(entryValue(s.args))
	switch {
	case s.command == nil && s.args == nil:
		insert(s.header.end, command+args)
	case s.command == nil:
		insert(s.header.end, command)
	case configured != o.Binary:
		replace(s.command, command)
	}
	switch {
	case s.args == nil && s.command != nil:
		insert(s.command.end, args)
	case s.args != nil && !slices.Equal(currentArgs, o.serverArgs()):
		replace(s.args, args)
	}
	// Later edits first, so the offsets of earlier ones stay right.
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, e := range edits {
		text = text[:e.start] + e.text + text[e.end:]
	}
	return []byte(text), nil
}

func entryValue(entry *tomlEntry) string {
	if entry == nil {
		return ""
	}
	return entry.value
}

// uninstalledConfig is config.toml without the flashheart MCP server's
// tables, and the blank line setup put before each.
func uninstalledConfig(data []byte) ([]byte, error) {
	s, err := findServer(data)
	if err != nil {
		return nil, err
	}
	text := string(data)
	for index := len(s.spans) - 1; index >= 0; index-- {
		start, end := s.spans[index][0], s.spans[index][1]
		if start >= 2 && text[start-2:start] == "\n\n" {
			start--
		}
		text = text[:start] + text[end:]
	}
	return []byte(text), nil
}

// configuredCommand is the server's command, resolved as the shell would,
// or "" when it is not a plain string.
func (o Options) configuredCommand(entry *tomlEntry) string {
	if entry == nil {
		return ""
	}
	command, ok := parseTOMLString(entry.value)
	if !ok {
		return ""
	}
	return o.resolve(command)
}

// configuredArgs is the server's arguments; ok is false when they are not
// a plain list of strings. Codex runs a server without args with none.
func configuredArgs(entry *tomlEntry) ([]string, bool) {
	if entry == nil {
		return []string{}, true
	}
	return parseTOMLStringArray(entry.value)
}
