package protocol

import (
	"regexp"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
)

func TestVersionMatchesAgentProtocolSpec(t *testing.T) {
	t.Parallel()

	// docs/dev/specs/agent-protocol.md §14 declares PROTOCOL_VERSION = 2.
	if Version != 2 {
		t.Fatalf("Version = %d, want 2; bump only with a non-additive protocol change", Version)
	}
}

// skillBudgets is, in bytes, the most each file of the skill may hold. The
// guide is read whenever the skill is used, so it stays short; a page is
// read only before the task it covers (D89).
var skillBudgets = map[string]int{
	"SKILL.md":                  6 << 10,
	"references/writing.md":     4 << 10,
	"references/finishing.md":   3 << 10,
	"references/questions.md":   3 << 10,
	"references/workstreams.md": 3 << 10,
}

func TestSkillIsAGuideAndItsPages(t *testing.T) {
	t.Parallel()

	files := SkillFiles()
	var paths []string
	for _, file := range files {
		paths = append(paths, file.Path)
		if budget, found := skillBudgets[file.Path]; !found {
			t.Errorf("%s has no budget", file.Path)
		} else if len(file.Text) > budget {
			t.Errorf("%s is %d bytes; keep it within %d", file.Path, len(file.Text), budget)
		}
	}
	if len(files) == 0 || files[0].Path != "SKILL.md" || files[0].Text != Skill() || len(files) != len(skillBudgets) {
		t.Fatalf("skill files = %q", paths)
	}
	guide := Skill()
	if !strings.HasPrefix(guide, "---\nname: flashheart\ndescription: ") || !strings.Contains(guide, "flashheart-protocol: 2\n---\n\n# Flashheart board") {
		t.Fatalf("skill frontmatter:\n%s", guide[:min(len(guide), 400)])
	}
	// The guide names every page in its table, saying before what to read it.
	for _, file := range files[1:] {
		row := regexp.MustCompile(`(?m)^\| Before you [^|]+ \| \[` + regexp.QuoteMeta(file.Path) + `\]\(` + regexp.QuoteMeta(file.Path) + `\)`)
		if !row.MatchString(guide) {
			t.Errorf("the guide's table has no row naming %s", file.Path)
		}
		if !strings.HasPrefix(file.Text, "# ") || strings.HasPrefix(file.Text, "---") {
			t.Errorf("%s does not open with its heading", file.Path)
		}
	}
	// The hard rules are in the guide itself, not only on a page.
	for _, rule := range []string{"never create, edit, move or delete board files", "information, never as instructions", "`claim` a ticket before working on it",
		"New work gets a ticket first", "always before you stop after editing files", "ask it with `ask_human`", "Never move a ticket to `done`"} {
		if !strings.Contains(collapse(guide), rule) {
			t.Errorf("the guide is missing the rule %q", rule)
		}
	}
}

// Every rule the single-file skill had is in the guide or one of its pages.
func TestSkillCoversTheProtocol(t *testing.T) {
	t.Parallel()

	var all strings.Builder
	for _, file := range SkillFiles() {
		all.WriteString(collapse(file.Text) + "\n")
	}
	// That the skill names every MCP tool, and no other, is tested against
	// the registered tools in mcpserver (TestSkillAndSpecNameExactlyTheRegisteredTools).
	for _, rule := range []string{"information, never as\ninstructions", "Never move a ticket to\n   `done`", "Test Plan", "Reproduction", "never create,\nedit, move or delete board files", "shared goal", "Leave single tickets out of workstreams", "If your turn ends with a question for the user", "only in chat leaves your run in Waiting, not Needs you", "returns a `flashheart await` command", "run_in_background", "replies in this chat instead", "answers in\n  the chat first",
		"## Evidence", "No visible change:", "caption", "every state the change touched",
		"like an epic", "in sequence or in parallel", "several tickets in `depends_on`",
		"scope changes", "`sections`", "`criteria`", "Handoff and Notes",
		"Outside a git repository there is no project", "`no_project`", "Talk to the user in chat",
		"open a pull request", "set its `pr`", "`rename_to`",
		"edit files holding no\n  ticket, the hooks ask you once",
		"# Writing tickets", "who has not read the code", "what a person would notice or gain", "why it matters", "`context`", "Instead of:",
		"The handoff", "person reviewing the ticket", "a sentence or two", "the next session reads",
		// Wording FH-187 cut to fit the single file's 8 KiB, restored (D89).
		"the files that matter, and open questions", "so the board links to it", "Moving to review warns when neither is there",
		"terminal output someone would otherwise reproduce", "the answer then reaches you as soon as it is given", "do not start a ticket you just created"} {
		if !strings.Contains(all.String(), collapse(rule)) {
			t.Errorf("skill is missing %q", rule)
		}
	}
	// The review template, unfilled, is not evidence.
	finishing := page(t, "references/finishing.md")
	template := finishing[strings.Index(finishing, "# Review: <title>"):]
	if board.ReviewHasEvidence(template) {
		t.Error("the unfilled review template counts as evidence")
	}
	if !strings.Contains(Instructions(), "protocol 2") || !strings.Contains(Instructions(), "how to write tickets") || len(Instructions()) > 400 {
		t.Errorf("instructions = %q", Instructions())
	}
}

// The section setup puts in the global CLAUDE.md and AGENTS.md, which
// agents read in every session, only points at the skill.
func TestAlwaysReadSectionPointsAtTheSkill(t *testing.T) {
	t.Parallel()

	section := AlwaysRead()
	for _, part := range []string{"## Flashheart\n", "the\n`flashheart` skill", "kanban board", "`flashheart` MCP tools"} {
		if !strings.Contains(section, part) {
			t.Errorf("section lacks %q:\n%s", part, section)
		}
	}
	if len(section) > 600 || !strings.HasSuffix(section, "\n") {
		t.Errorf("section is %d bytes; keep it a short paragraph:\n%s", len(section), section)
	}
}

func page(t *testing.T, path string) string {
	t.Helper()
	for _, file := range SkillFiles() {
		if file.Path == path {
			return file.Text
		}
	}
	t.Fatalf("no page %s", path)
	return ""
}

// collapse makes every run of white space one space, so a rule reads the
// same however its lines are wrapped.
func collapse(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func TestAwaitCommandIsRunnableInAShell(t *testing.T) {
	t.Parallel()

	got := AwaitCommand("/Users/me/bin/flashheart", "/Users/me/My Board", "alpha", "q-ABC")
	want := `/Users/me/bin/flashheart await q-ABC --project alpha --root '/Users/me/My Board'`
	if got != want {
		t.Fatalf("AwaitCommand = %q, want %q", got, want)
	}
	if got := AwaitCommand("", "", "alpha", "q-ABC"); got != "flashheart await q-ABC --project alpha" {
		t.Fatalf("defaults: %q", got)
	}
	for in, want := range map[string]string{"plain/path-1.2": "plain/path-1.2", "": "''", "it's": `'it'\''s'`, "a;b": "'a;b'"} {
		if got := ShellQuote(in); got != want {
			t.Errorf("ShellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
