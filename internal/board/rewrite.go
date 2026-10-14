package board

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Rewrites: an agent correcting a ticket's text when its scope changes
// (FH-69, D75). An agent replaces whole sections it names (not Handoff or
// Notes, and not the criteria, which it edits by operation so ticks are
// kept), and store.ApplyTicketEdit keeps the text a rewrite replaced in
// the ticket's files, worked out from the ticket before and after it.

var (
	// ErrCriteriaSection refuses rewriting the acceptance criteria whole:
	// they are edited by operation, which keeps each criterion's tick.
	ErrCriteriaSection = errors.New("acceptance criteria are edited by operation (add, reword, remove, move), which keeps their ticks")
	// ErrUnknownSection refuses a section the ticket does not have and
	// the template does not name, so a misspelt name adds nothing.
	ErrUnknownSection = errors.New("the ticket has no such section")
)

// agentTemplateSections are the template sections an agent may rewrite,
// adding them when the ticket lacks them.
var agentTemplateSections = []string{SectionDescription, SectionTestPlan, SectionReproduction, SectionContext}

// NewAgentSectionChange checks an agent's rewrite of one section of the
// ticket in data: NewSectionChange's rules, the criteria refused
// (ErrCriteriaSection), and a section that is not in the template only
// when the ticket has it (ErrUnknownSection, naming the sections that may
// be rewritten). A missing template section is added in its place.
func NewAgentSectionChange(data []byte, name, content string) (SectionChange, error) {
	change, err := NewSectionChange(name, content)
	if err != nil {
		return SectionChange{}, err
	}
	if change.name == SectionCriteria {
		return SectionChange{}, fmt.Errorf("%s: %w", change.name, ErrCriteriaSection)
	}
	if change.field != "section" {
		return change, nil
	}
	rewritable := slices.Clone(agentTemplateSections)
	for _, section := range mdfile.Sections(mdfile.Parse(data).Body) {
		heading := strings.TrimSpace(section.Heading)
		if strings.EqualFold(heading, change.name) {
			change.name = heading
			return change, nil
		}
		if !slices.ContainsFunc(templateSections, func(known templateSection) bool { return strings.EqualFold(known.name, heading) }) &&
			!slices.Contains(rewritable, heading) {
			rewritable = append(rewritable, heading)
		}
	}
	return SectionChange{}, fmt.Errorf("%q: %w; the sections that can be rewritten are %s", change.name, ErrUnknownSection, strings.Join(rewritable, ", "))
}

// Criteria operations (CriterionOp.Kind).
const (
	CriterionAdd    = "add"
	CriterionReword = "reword"
	CriterionRemove = "remove"
	CriterionMove   = "move"
)

// MaxCriterion is the most characters a criterion written by operation
// has.
const MaxCriterion = 500

// CriterionOp is one edit of a ticket's acceptance criteria.
type CriterionOp struct {
	// Kind is CriterionAdd, CriterionReword, CriterionRemove or
	// CriterionMove.
	Kind string
	// Item is the criterion reworded, removed or moved: its 0-based
	// index in the ticket before the edit, whatever operations came first.
	Item int
	// Text is an added or reworded criterion's text, one line.
	Text string
	// Position is where an added or moved criterion goes, 1-based, in
	// the list as the operations before it left it; 0 adds at the end.
	Position int
}

// CriteriaEdit is a checked list of criteria operations for a ticket with
// a given number of criteria. Only NewCriteriaEdit makes one.
type CriteriaEdit struct {
	count int
	ops   []CriterionOp
}

// NewCriteriaEdit checks ops against a ticket with count criteria, in
// order: every criterion named exists and was not removed by an earlier
// operation, every position is in the list as it then stands, and added
// and reworded text is not empty.
func NewCriteriaEdit(count int, ops []CriterionOp) (CriteriaEdit, error) {
	if len(ops) == 0 {
		return CriteriaEdit{}, errors.New("no criteria operations")
	}
	list := make([]int, count)
	for index := range list {
		list[index] = index
	}
	checked := make([]CriterionOp, 0, len(ops))
	for number, op := range ops {
		op.Text = OneLine(op.Text, MaxCriterion)
		fail := func(format string, args ...any) (CriteriaEdit, error) {
			return CriteriaEdit{}, fmt.Errorf("criteria operation %d (%s): %s", number+1, op.Kind, fmt.Sprintf(format, args...))
		}
		at := -1
		if op.Kind != CriterionAdd {
			switch {
			case op.Kind != CriterionReword && op.Kind != CriterionRemove && op.Kind != CriterionMove:
				return CriteriaEdit{}, fmt.Errorf("criteria operation %d: %q is not an operation; use add, reword, remove or move", number+1, op.Kind)
			case op.Item < 0 || op.Item >= count:
				return fail("there is no criterion %d; number them 1 to %d as the ticket has them", op.Item+1, count)
			}
			if at = slices.Index(list, op.Item); at < 0 {
				return fail("criterion %d is removed earlier in this call", op.Item+1)
			}
		}
		if (op.Kind == CriterionAdd || op.Kind == CriterionReword) && op.Text == "" {
			return fail("a criterion needs its text")
		}
		switch op.Kind {
		case CriterionAdd:
			if op.Position == 0 {
				op.Position = len(list) + 1
			}
			if op.Position < 1 || op.Position > len(list)+1 {
				return fail("add at a position 1 to %d", len(list)+1)
			}
			list = slices.Insert(list, op.Position-1, count+number)
		case CriterionRemove:
			list = slices.Delete(list, at, at+1)
		case CriterionMove:
			if op.Position < 1 || op.Position > len(list) {
				return fail("move criterion %d to a position 1 to %d", op.Item+1, len(list))
			}
			list = slices.Insert(slices.Delete(list, at, at+1), op.Position-1, op.Item)
		}
		checked = append(checked, op)
	}
	return CriteriaEdit{count: count, ops: checked}, nil
}

// Apply edits the criteria section of a ticket's text: a reworded
// criterion keeps its box and loses its old continuation lines, a moved
// one keeps all its lines, an added one is unticked and uses the list's
// bullet. Text around the list is kept; a placeholder such as "None."
// gives way to the first criterion, and a missing section is added in the
// template's place. Every other byte of the file stays as it was.
func (e CriteriaEdit) Apply(data []byte) ([]byte, error) {
	section, _ := mdfile.FindSection(mdfile.Parse(data).Body, SectionCriteria)
	head, items, tail := mdfile.TaskItems(strings.ReplaceAll(section.Content, "\r\n", "\n"))
	if len(items) != e.count {
		return nil, fmt.Errorf("the ticket has %d criteria, not the %d these operations were checked against", len(items), e.count)
	}
	type block struct {
		text string
		item int
	}
	blocks := make([]block, len(items))
	for index, text := range items {
		blocks[index] = block{text: text, item: index}
	}
	bullet := "-"
	if len(items) > 0 {
		bullet = items[0][:1]
	}
	find := func(item int) int {
		return slices.IndexFunc(blocks, func(b block) bool { return b.item == item })
	}
	for number, op := range e.ops {
		switch op.Kind {
		case CriterionAdd:
			blocks = slices.Insert(blocks, op.Position-1, block{text: bullet + " [ ] " + op.Text + "\n", item: e.count + number})
		case CriterionReword:
			at := find(op.Item)
			first, _, _ := strings.Cut(blocks[at].text, "\n")
			// "- [x] " is the bullet and box the criterion keeps.
			blocks[at].text = first[:6] + op.Text + "\n" + trailingBlankLines(blocks[at].text)
		case CriterionRemove:
			at := find(op.Item)
			blocks = slices.Delete(blocks, at, at+1)
		case CriterionMove:
			at := find(op.Item)
			moved := blocks[at]
			blocks = slices.Insert(slices.Delete(blocks, at, at+1), op.Position-1, moved)
		}
	}
	if len(items) == 0 {
		switch trimmed := strings.TrimSpace(head); {
		case trimmed == "" || mdfile.IsPlaceholder(trimmed):
			head = ""
		default:
			head = strings.TrimRight(head, " \t\n") + "\n\n"
		}
	}
	var content strings.Builder
	content.WriteString(head)
	for _, b := range blocks {
		content.WriteString(b.text)
		if !strings.HasSuffix(b.text, "\n") {
			content.WriteString("\n")
		}
	}
	content.WriteString(tail)
	change, err := NewSectionChange(SectionCriteria, content.String())
	if err != nil {
		return nil, err
	}
	return change.Apply(data)
}

// trailingBlankLines is the blank lines that end a criterion's lines, so
// a reworded criterion keeps its place in a loose list.
func trailingBlankLines(text string) string {
	lines := strings.SplitAfter(text, "\n")
	blank := 0
	for index := len(lines) - 1; index > 0; index-- {
		switch {
		case lines[index] == "" && index == len(lines)-1:
		case strings.TrimSpace(lines[index]) == "":
			blank++
		default:
			return strings.Repeat("\n", blank)
		}
	}
	return strings.Repeat("\n", blank)
}

// Summary says what the operations did, numbering criteria as the ticket
// had them: "criteria: 2 reworded, 3 removed, 4 moved to 1, one added".
func (e CriteriaEdit) Summary() string {
	parts := make([]string, 0, len(e.ops))
	for _, op := range e.ops {
		switch op.Kind {
		case CriterionAdd:
			parts = append(parts, fmt.Sprintf("one added as %d", op.Position))
		case CriterionReword:
			parts = append(parts, fmt.Sprintf("%d reworded", op.Item+1))
		case CriterionRemove:
			parts = append(parts, fmt.Sprintf("%d removed", op.Item+1))
		case CriterionMove:
			parts = append(parts, fmt.Sprintf("%d moved to %d", op.Item+1, op.Position))
		}
	}
	return "criteria: " + strings.Join(parts, ", ")
}

// Rewritten compares a ticket before and after a rewrite: changed names
// the sections whose text changed (added and removed ones too) in the
// new file's order, and earlier is the old text of those that had any,
// as a person would restore it. Notes and Handoff are never counted.
func Rewritten(before, after []byte) (changed []string, earlier []mdfile.Section) {
	old, now := mdfile.Sections(mdfile.Parse(before).Body), mdfile.Sections(mdfile.Parse(after).Body)
	text := func(list []mdfile.Section, heading string) (mdfile.Section, string, bool) {
		at := slices.IndexFunc(list, func(s mdfile.Section) bool {
			return strings.EqualFold(strings.TrimSpace(s.Heading), heading)
		})
		if at < 0 {
			return mdfile.Section{}, "", false
		}
		return list[at], strings.TrimSpace(strings.ReplaceAll(list[at].Content, "\r\n", "\n")), true
	}
	consider := func(heading string) {
		heading = strings.TrimSpace(heading)
		if strings.EqualFold(heading, SectionNotes) || strings.EqualFold(heading, SectionHandoff) ||
			slices.ContainsFunc(changed, func(name string) bool { return strings.EqualFold(name, heading) }) {
			return
		}
		was, oldText, hadIt := text(old, heading)
		_, newText, hasIt := text(now, heading)
		if hadIt == hasIt && oldText == newText {
			return
		}
		changed = append(changed, heading)
		if oldText != "" {
			earlier = append(earlier, mdfile.Section{Heading: heading, Content: was.Content})
		}
	}
	for _, section := range now {
		consider(section.Heading)
	}
	for _, section := range old {
		consider(section.Heading)
	}
	return changed, earlier
}

// EarlierText is the markdown file that keeps the sections a rewrite
// replaced, each under its own heading as it was, for a person to copy
// back: who replaced them and when head it.
func EarlierText(id, by string, at time.Time, sections []mdfile.Section) []byte {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s: text replaced by %s at %s\n\n", id, OneLine(by, MaxTitle), at.UTC().Format(time.RFC3339))
	out.WriteString("Each section below is as it was before the rewrite. To restore one, copy its text back into that section of the ticket.\n")
	for _, section := range sections {
		content := strings.TrimRight(strings.ReplaceAll(section.Content, "\r\n", "\n"), " \t\n")
		content = strings.TrimLeft(content, "\n")
		fmt.Fprintf(&out, "\n## %s\n\n%s\n", section.Heading, content)
	}
	return []byte(out.String())
}
