package mcpserver

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/store"
)

// listTools returns the tools the server registers, as a client sees them.
func listTools(t *testing.T) []*mcp.Tool {
	t.Helper()
	e := newEnv(t, "DM")
	e.connect()
	tools, err := e.client.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return tools.Tools
}

// inputs is the names of a tool's input properties.
func inputs(t *testing.T, tool *mcp.Tool) []string {
	t.Helper()
	data, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	var names []string
	for name := range schema.Properties {
		names = append(names, name)
	}
	return names
}

// specTools is the first column of agent-protocol §7.2's tools table.
func specTools(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("../../docs/dev/specs/agent-protocol.md")
	if err != nil {
		t.Fatal(err)
	}
	spec := string(data)
	start := strings.Index(spec, "\n### 7.2 Tools\n")
	if start < 0 {
		t.Fatal("agent-protocol.md has no §7.2 Tools")
	}
	section := spec[start+1:]
	if end := strings.Index(section, "\n### "); end >= 0 {
		section = section[:end]
	}
	var names []string
	for _, row := range regexp.MustCompile("(?m)^\\| `([a-z_]+)` \\|").FindAllStringSubmatch(section, -1) {
		names = append(names, row[1])
	}
	if len(names) == 0 {
		t.Fatal("§7.2 has no tool rows")
	}
	return names
}

// The skill and the spec are hand-kept copies of the tool list; each must
// name every registered tool and no other (AGENTS.md: the protocol, the
// skill and the tests move together).
func TestSkillAndSpecNameExactlyTheRegisteredTools(t *testing.T) {
	t.Parallel()

	tools := listTools(t)
	registered := map[string]bool{}
	// Words the skill puts in backticks that are not tools: the tools'
	// inputs, the ticket fields update_ticket sets, ticket values, the
	// server's name, an error code and Claude Code's background flag.
	words := map[string]bool{Name: true, "no_project": true, "run_in_background": true}
	for _, field := range settable {
		words[field] = true
	}
	for _, tool := range tools {
		registered[tool.Name] = true
		for _, input := range inputs(t, tool) {
			words[input] = true
		}
	}
	for _, value := range slices.Concat(board.TicketTypes, board.Priorities) {
		words[value] = true
	}
	for _, column := range board.Columns {
		words[string(column)] = true
	}

	spec := specTools(t)
	for name := range registered {
		if !slices.Contains(spec, name) {
			t.Errorf("agent-protocol §7.2 has no row for %s", name)
		}
	}
	for _, name := range spec {
		if !registered[name] {
			t.Errorf("agent-protocol §7.2 has a row for %s, which is not registered", name)
		}
	}

	skill := protocol.Skill()
	named := map[string]bool{}
	for _, m := range regexp.MustCompile("`([a-z][a-z_]*)[` ]").FindAllStringSubmatch(skill, -1) {
		named[m[1]] = true
	}
	for name := range registered {
		if !named[name] {
			t.Errorf("the skill does not name %s", name)
		}
	}
	for name := range named {
		if !registered[name] && !words[name] {
			t.Errorf("the skill names `%s`, which is neither a tool nor one of their inputs", name)
		}
	}
}

// update_ticket's description lists the fields its set input accepts.
func TestUpdateTicketDescribesEverySettableField(t *testing.T) {
	t.Parallel()

	tools := listTools(t)
	i := slices.IndexFunc(tools, func(tool *mcp.Tool) bool { return tool.Name == "update_ticket" })
	if i < 0 {
		t.Fatal("update_ticket is not registered")
	}
	description := tools[i].Description
	list := regexp.MustCompile(`set fields \(([^)]*)\)`).FindStringSubmatch(description)
	if list == nil {
		t.Fatalf("description has no set fields list: %q", description)
	}
	described := strings.Split(list[1], ", ")
	for _, field := range settable {
		if !slices.Contains(described, field) {
			t.Errorf("update_ticket's description does not list %s: %q", field, description)
		}
	}
	for _, field := range described {
		if !slices.Contains(settable, field) {
			t.Errorf("update_ticket's description lists %s, which it cannot set", field)
		}
	}
}

// How Claude Code runs the await command is for Claude runs; a Codex
// session, with hooks or without, is not told about Claude Code (§7.5).
func TestAskHumanGivesClaudeCodeAdviceOnlyToClaude(t *testing.T) {
	t.Parallel()

	ask := map[string]any{"kind": "question", "text": "Which schema?"}
	hookless := newEnv(t, "DM")
	hookless.ticket(store.NewTicket{Title: "A"})
	hooked := newEnv(t, "DM")
	hooked.ticket(store.NewTicket{Title: "A"})
	hooked.codexSession("codex:019a0000-hooks", "feature/demo")
	for name, out := range map[string]string{
		"hookless": hookless.okOn(hookless.dial(codexClient), "ask_human", ask),
		"hooked":   hooked.okOn(hooked.dial(codexClient), "ask_human", ask),
	} {
		contains(t, out, "\nflashheart await q-")
		if strings.Contains(out, "Claude Code") || strings.Contains(out, "run_in_background") {
			t.Errorf("%s Codex session was given Claude Code advice:\n%s", name, out)
		}
	}
	claude := newEnv(t, "DM")
	claude.startSession(session)
	contains(t, claude.ok("ask_human", ask), "Claude Code: Bash with run_in_background")
}

// update_ticket takes addressed, the change requests an agent marks done
// (FH-126); its description, the §7.2 row and the skill say so.
func TestUpdateTicketTakesAddressedChangeRequests(t *testing.T) {
	t.Parallel()

	tools := listTools(t)
	i := slices.IndexFunc(tools, func(tool *mcp.Tool) bool { return tool.Name == "update_ticket" })
	if i < 0 {
		t.Fatal("update_ticket is not registered")
	}
	if !slices.Contains(inputs(t, tools[i]), "addressed") {
		t.Errorf("update_ticket's inputs = %v, want addressed", inputs(t, tools[i]))
	}
	if !strings.Contains(tools[i].Description, "addressed") {
		t.Errorf("update_ticket's description does not mention addressed: %q", tools[i].Description)
	}
	data, err := os.ReadFile("../../docs/dev/specs/agent-protocol.md")
	if err != nil {
		t.Fatal(err)
	}
	row := regexp.MustCompile("(?m)^\\| `update_ticket` \\|.*$").FindString(string(data))
	if !strings.Contains(row, "`addressed?`") {
		t.Errorf("agent-protocol §7.2's update_ticket row does not list addressed?: %q", row)
	}
	if !strings.Contains(protocol.Skill(), "`addressed`") {
		t.Error("the skill does not say how to mark a change request addressed")
	}
}

// toolListBudget is, in bytes, the most every agent session may receive
// before it does any work: the tools/list result (each tool's name,
// description and input schema, as JSON) and the server's instructions,
// which initialize sends to every session. Agent-protocol §7.2 records the
// same number (D52, FH-134). Raise both on purpose, saying why in the pull
// request, after checking the ability could not extend an existing tool.
const toolListBudget = 16500

// toolSize is one tool's share of the tool list.
type toolSize struct {
	name  string
	bytes int
}

// tokens estimates a size in tokens: about four bytes a token, no tokenizer.
func tokens(bytes int) int { return (bytes + 3) / 4 }

// Every tool's name, description and schema is in every agent's context
// before it does any work (D52), so the list's size is measured, reported
// (go test -v -run TestToolListStaysWithinItsBudget ./internal/mcpserver)
// and held to a budget.
func TestToolListStaysWithinItsBudget(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	e.connect()
	tools, err := e.client.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if tools.NextCursor != "" {
		t.Fatal("tools/list is paginated; measure every page")
	}
	listed, err := json.Marshal(&mcp.ListToolsResult{Tools: tools.Tools})
	if err != nil {
		t.Fatal(err)
	}
	instructions := len(e.client.InitializeResult().Instructions)
	total := len(listed) + instructions

	var sizes []toolSize
	for _, tool := range tools.Tools {
		data, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		sizes = append(sizes, toolSize{tool.Name, len(data)})
	}
	slices.SortFunc(sizes, func(a, b toolSize) int { return b.bytes - a.bytes })
	var report strings.Builder
	for _, size := range sizes {
		fmt.Fprintf(&report, "  %-18s %6d bytes  ~%5d tokens\n", size.name, size.bytes, tokens(size.bytes))
	}
	t.Logf("%d tools: tools/list %d bytes (~%d tokens), instructions %d bytes, %d bytes (~%d tokens) in all; budget %d bytes\n%s",
		len(sizes), len(listed), tokens(len(listed)), instructions, total, tokens(total), toolListBudget, report.String())

	if total > toolListBudget {
		var largest []string
		for _, size := range sizes[:min(5, len(sizes))] {
			largest = append(largest, fmt.Sprintf("%s %d bytes", size.name, size.bytes))
		}
		t.Errorf("the tool list and instructions are %d bytes (~%d tokens), over the budget of %d bytes (D52, agent-protocol §7.2).\n"+
			"Largest tools: %s.\n"+
			"Every agent reads all of it before any work: first try an optional input on an existing tool, or shorter descriptions. "+
			"To raise the budget on purpose, set toolListBudget in internal/mcpserver/toollist_test.go and the budget in agent-protocol §7.2 "+
			"to the new size plus about 5%%, and say why in the pull request.",
			total, tokens(total), toolListBudget, strings.Join(largest, ", "))
	}

	data, err := os.ReadFile("../../docs/dev/specs/agent-protocol.md")
	if err != nil {
		t.Fatal(err)
	}
	recorded := regexp.MustCompile(`tool list's budget is ([\d,]+) bytes`).FindStringSubmatch(string(data))
	if recorded == nil {
		t.Fatal("agent-protocol §7.2 does not record the tool list's budget")
	}
	if got := strings.ReplaceAll(recorded[1], ",", ""); got != strconv.Itoa(toolListBudget) {
		t.Errorf("agent-protocol §7.2 records a budget of %s bytes; toolListBudget is %d", recorded[1], toolListBudget)
	}
}
