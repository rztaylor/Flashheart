package board

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// A ticket's activity file (board-format §Activity, D47): people's
// comments and the ticket's history, one dated, typed entry each, oldest
// first, in activity.md beside the ticket file. Entries are appended and
// a comment is edited in place; every other byte is kept as written, so
// entries of kinds this version does not read survive.

// ActivityFile is the activity file's name in the ticket's folder.
const ActivityFile = "activity.md"

// Entry kinds this version reads.
const (
	EntryComment = "comment"
	EntryHistory = "history"
)

// Limits of what an entry holds. A comment's text has the section limit
// (MaxSectionText).
const (
	// MaxQuote is the most bytes of the ticket a comment quotes.
	MaxQuote = 4 << 10
	// MaxQuoteFrom is the most characters naming where a quote is from.
	MaxQuoteFrom = 200
	// MaxAuthor is the most characters of an entry's author.
	MaxAuthor = 300
)

// CommentState is where a comment stands in a review: open until it is
// marked as a change requested, which an agent marks addressed and the
// person resolves (FH-126).
type CommentState string

// Comment states.
const (
	CommentOpen            CommentState = "open"
	CommentChangeRequested CommentState = "change-requested"
	CommentAddressed       CommentState = "addressed"
	CommentResolved        CommentState = "resolved"
)

// CommentStates lists every state, in the order a comment moves through
// them.
var CommentStates = []CommentState{CommentOpen, CommentChangeRequested, CommentAddressed, CommentResolved}

// ErrNoComment reports a comment id the activity file does not have.
var ErrNoComment = errors.New("no such comment")

// Comment is a person's comment on a ticket. Edited is zero until the
// comment is edited; Quote is the part of the ticket it quotes ("" for
// none) and QuoteFrom where that part is.
type Comment struct {
	ID        string
	Time      time.Time
	By        string
	State     CommentState
	Edited    time.Time
	QuoteFrom string
	Quote     string
	Body      string
}

// History is one write that changed the ticket: who, when and what, never
// the ticket's text.
type History struct {
	ID      string
	Time    time.Time
	By      string
	Changes []Change
}

// What a history change records.
const (
	ChangeCreated  = "created"
	ChangeMoved    = "moved"
	ChangeEdited   = "edited"
	ChangeTicked   = "ticked"
	ChangeUnticked = "unticked"
	ChangeArchived = "archived"
	ChangeRestored = "restored"
)

// Change is one thing a write changed: created (To: the status it starts
// in), moved (From, To), edited (Fields: frontmatter keys, title, section
// names), ticked or unticked (Criteria, Steps: 1-based numbers of
// acceptance criteria and review steps), archived or restored.
type Change struct {
	What     string
	From, To string
	Fields   []string
	Criteria []int
	Steps    []int
}

// Activity is a parsed activity file. Unknown counts entries this version
// does not read (another kind, or a heading it cannot date); they are
// kept as written.
type Activity struct {
	Comments []Comment
	History  []History
	Unknown  int

	preamble string
	entries  []entry
}

// entry is one entry's bytes, from its heading to the next.
type entry struct {
	raw  string
	kind string
	id   string
}

var (
	entryHeading = regexp.MustCompile(`^### (\S+)` + regexp.QuoteMeta(dot) + `([a-z][a-z0-9-]{0,39})` +
		regexp.QuoteMeta(dot) + `(.+)` + regexp.QuoteMeta(dot) + `([0-9A-Za-z_-]{1,64})[ \t]*$`)
	attributeLine = regexp.MustCompile(`^([a-z][a-z-]{0,39}):(?: (.*))?$`)
	changeLine    = regexp.MustCompile(`^- ([a-z][a-z-]{0,39})(?:: (.*))?$`)
	runAuthor     = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}:\S+$`)
)

const timeLayout = "2006-01-02T15:04:05Z"

// ParseActivity reads an activity file. It never fails: a missing or empty
// file is no activity, and what it cannot read is kept and counted.
func ParseActivity(data []byte) Activity {
	text := string(data)
	var activity Activity
	var starts []int
	fence, offset := "", 0
	for index, line := range strings.SplitAfter(text, "\n") {
		content := strings.TrimRight(line, "\r\n")
		if index == 0 {
			content = strings.TrimPrefix(content, "\ufeff")
		}
		if fence == "" && entryHeading.MatchString(content) {
			starts = append(starts, offset)
		}
		fence = nextFence(fence, content)
		offset += len(line)
	}
	if len(starts) == 0 {
		activity.preamble = text
		return activity
	}
	activity.preamble = text[:starts[0]]
	for index, start := range starts {
		end := len(text)
		if index+1 < len(starts) {
			end = starts[index+1]
		}
		activity.add(text[start:end])
	}
	return activity
}

// nextFence is the code fence open after line, given the one open before.
func nextFence(fence, line string) string {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 3 {
		return fence
	}
	switch marker := trimmed[:3]; {
	case marker != "```" && marker != "~~~":
		return fence
	case fence == "":
		return marker
	case fence == marker:
		return ""
	}
	return fence
}

// add reads one entry and keeps its bytes.
func (a *Activity) add(raw string) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	match := entryHeading.FindStringSubmatch(strings.TrimPrefix(lines[0], "\ufeff"))
	e := entry{raw: raw, kind: match[2], id: match[4]}
	a.entries = append(a.entries, e)
	at, err := time.Parse(time.RFC3339, match[1])
	if err != nil {
		a.Unknown++
		return
	}
	by, rest := strings.TrimSpace(match[3]), lines[1:]
	switch e.kind {
	case EntryComment:
		comment, _ := parseComment(rest)
		comment.ID, comment.Time, comment.By = e.id, at.UTC(), by
		a.Comments = append(a.Comments, comment)
	case EntryHistory:
		a.History = append(a.History, History{ID: e.id, Time: at.UTC(), By: by, Changes: parseChanges(rest)})
	default:
		a.Unknown++
	}
}

// Bytes is the file as it was read, with any edits.
func (a Activity) Bytes() []byte {
	var out strings.Builder
	out.WriteString(a.preamble)
	for _, e := range a.entries {
		out.WriteString(e.raw)
	}
	return []byte(out.String())
}

// IDs lists every entry's id, so a writer can choose one not in use.
func (a Activity) IDs() []string {
	ids := make([]string, 0, len(a.entries))
	for _, e := range a.entries {
		ids = append(ids, e.id)
	}
	return ids
}

// attribute is one "key: value" line at the top of a comment.
type attribute struct{ key, value string }

// parseComment reads a comment's lines after its heading: its attributes
// (always written, at least its state), the quote when one is named, then
// its text.
func parseComment(lines []string) (Comment, []attribute) {
	comment := Comment{State: CommentOpen}
	lines = trimBlank(lines)
	var attributes []attribute
	end := slices.IndexFunc(lines, blank)
	if end < 0 {
		end = len(lines)
	}
	if end > 0 && !slices.ContainsFunc(lines[:end], func(line string) bool { return !attributeLine.MatchString(line) }) {
		for _, line := range lines[:end] {
			match := attributeLine.FindStringSubmatch(line)
			attributes = append(attributes, attribute{key: match[1], value: strings.TrimSpace(match[2])})
		}
		lines = trimBlank(lines[end:])
	}
	quoted := false
	for _, attr := range attributes {
		switch attr.key {
		case "state":
			if state := CommentState(attr.value); slices.Contains(CommentStates, state) {
				comment.State = state
			}
		case "edited":
			if edited, err := time.Parse(time.RFC3339, attr.value); err == nil {
				comment.Edited = edited.UTC()
			}
		case "quote":
			quoted, comment.QuoteFrom = true, attr.value
		}
	}
	if quoted {
		var quote []string
		for len(lines) > 0 && strings.HasPrefix(lines[0], ">") {
			quote = append(quote, strings.TrimPrefix(strings.TrimPrefix(lines[0], ">"), " "))
			lines = lines[1:]
		}
		comment.Quote = strings.Join(quote, "\n")
		lines = trimBlank(lines)
	}
	comment.Body = strings.Join(lines, "\n")
	return comment, attributes
}

func blank(line string) bool { return strings.TrimSpace(line) == "" }

// trimBlank drops blank lines at both ends.
func trimBlank(lines []string) []string {
	for len(lines) > 0 && blank(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && blank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// parseChanges reads a history entry's bullets; one it does not know is
// left out (and kept in the file).
func parseChanges(lines []string) []Change {
	var changes []Change
	for _, line := range lines {
		match := changeLine.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		change, argument := Change{What: match[1]}, strings.TrimSpace(match[2])
		switch change.What {
		case ChangeCreated:
			change.To = argument
		case ChangeMoved:
			from, to, ok := strings.Cut(argument, "→")
			if !ok {
				from, to, _ = strings.Cut(argument, "->")
			}
			change.From, change.To = strings.TrimSpace(from), strings.TrimSpace(to)
		case ChangeEdited:
			change.Fields = splitList(argument)
		case ChangeTicked, ChangeUnticked:
			for _, item := range splitList(argument) {
				if number, ok := strings.CutPrefix(item, "criterion "); ok {
					if n, err := strconv.Atoi(number); err == nil {
						change.Criteria = append(change.Criteria, n)
					}
				} else if number, ok := strings.CutPrefix(item, "review step "); ok {
					if n, err := strconv.Atoi(number); err == nil {
						change.Steps = append(change.Steps, n)
					}
				}
			}
		case ChangeArchived, ChangeRestored:
		default:
			continue
		}
		changes = append(changes, change)
	}
	return changes
}

func splitList(text string) []string {
	var items []string
	for _, item := range strings.Split(text, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// Author is how an entry names who wrote it: one line, without the
// heading's separator, at most MaxAuthor characters; "unknown" when empty.
func Author(by string) string {
	if by = OneLine(strings.ReplaceAll(by, "·", "-"), MaxAuthor); by == "" {
		return "unknown"
	}
	return by
}

// ByPerson reports whether an entry's author is a person: not an agent's
// run id (agent:session) and not "unknown".
func ByPerson(by string) bool {
	return by != "unknown" && !runAuthor.MatchString(by)
}

// CheckComment validates a comment's text, the part of the ticket it
// quotes and where that is from, and returns them as they are written:
// the text without blank lines at its ends, its headings below the
// entries' and a code fence it leaves open closed (FH-4), and from on one
// line ("" without a quote).
func CheckComment(body, quote, from string) (string, string, string, error) {
	body = strings.Join(trimBlank(strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")), "\n")
	quote = strings.Join(trimBlank(strings.Split(strings.ReplaceAll(quote, "\r\n", "\n"), "\n")), "\n")
	switch {
	case body == "":
		return "", "", "", errors.New("a comment needs some text")
	case len(body) > MaxSectionText:
		return "", "", "", fmt.Errorf("a comment is limited to %d KiB", MaxSectionText>>10)
	case len(quote) > MaxQuote:
		return "", "", "", fmt.Errorf("a quote is limited to %d KiB", MaxQuote>>10)
	}
	if quote == "" {
		from = ""
	}
	return strings.TrimRight(mdfile.HeadingsFrom(body, 4), "\n"), quote, OneLine(from, MaxQuoteFrom), nil
}

// AppendComment appends a comment entry, starting the file (titled with
// the ticket's id) when data is empty. The comment is written as given:
// CheckComment has checked its text.
func AppendComment(data []byte, ticket string, c Comment) []byte {
	return appendEntry(data, ticket, renderComment(c, nil))
}

// AppendHistory appends a history entry, starting the file when data is
// empty.
func AppendHistory(data []byte, ticket string, h History) []byte {
	lines := []string{heading(h.Time, EntryHistory, h.By, h.ID), ""}
	for _, change := range h.Changes {
		lines = append(lines, change.line())
	}
	return appendEntry(data, ticket, lines)
}

// EditComment applies edit to comment id and rewrites that entry alone,
// keeping its heading, its attributes this version does not know and the
// blank lines after it; every other byte stays. A missing id, or one that
// is not a comment, is ErrNoComment.
func EditComment(data []byte, id string, edit func(*Comment)) ([]byte, Comment, error) {
	activity := ParseActivity(data)
	for index, e := range activity.entries {
		if e.kind != EntryComment || e.id != id {
			continue
		}
		head, rest, _ := strings.Cut(e.raw, "\n")
		lines := strings.Split(strings.ReplaceAll(rest, "\r\n", "\n"), "\n")
		at := slices.IndexFunc(activity.Comments, func(c Comment) bool { return c.ID == id })
		if at < 0 {
			break
		}
		comment, attributes := parseComment(lines)
		comment.ID, comment.Time, comment.By = id, activity.Comments[at].Time, activity.Comments[at].By
		edit(&comment)
		newline := "\n"
		if strings.HasSuffix(head, "\r") {
			newline = "\r\n"
		}
		body := renderComment(comment, attributes)
		body[0] = strings.TrimSuffix(head, "\r")
		core := strings.TrimRight(e.raw, " \t\r\n")
		activity.entries[index].raw = strings.Join(body, newline) + e.raw[len(core):]
		activity.Comments[at] = comment
		return activity.Bytes(), comment, nil
	}
	return nil, Comment{}, fmt.Errorf("comment %s: %w", id, ErrNoComment)
}

// heading is an entry's first line.
func heading(at time.Time, kind, by, id string) string {
	return "### " + at.UTC().Format(timeLayout) + dot + kind + dot + Author(by) + dot + id
}

// renderComment is a comment entry's lines: heading, attributes (state,
// edited, quote, then those kept from the file), quote, text.
func renderComment(c Comment, kept []attribute) []string {
	state := c.State
	if state == "" {
		state = CommentOpen
	}
	attributes := setAttribute(slices.Clone(kept), "state", string(state), true)
	attributes = setAttribute(attributes, "edited", c.Edited.UTC().Format(timeLayout), !c.Edited.IsZero())
	attributes = setAttribute(attributes, "quote", OneLine(c.QuoteFrom, MaxQuoteFrom), c.Quote != "")
	lines := []string{heading(c.Time, EntryComment, c.By, c.ID), ""}
	for _, attr := range attributes {
		lines = append(lines, strings.TrimRight(attr.key+": "+attr.value, " "))
	}
	lines = append(lines, "")
	if c.Quote != "" {
		for _, line := range strings.Split(c.Quote, "\n") {
			lines = append(lines, strings.TrimRight("> "+line, " "))
		}
		lines = append(lines, "")
	}
	return append(lines, strings.Split(c.Body, "\n")...)
}

// attributeOrder is the order Flashheart writes the attributes it knows.
var attributeOrder = []string{"state", "edited", "quote"}

// setAttribute sets key to value where it is, or adds it after the known
// attributes written before it; with keep false it removes key.
func setAttribute(attributes []attribute, key, value string, keep bool) []attribute {
	at := slices.IndexFunc(attributes, func(a attribute) bool { return a.key == key })
	switch {
	case !keep && at >= 0:
		return slices.Delete(attributes, at, at+1)
	case !keep:
		return attributes
	case at >= 0:
		attributes[at].value = value
		return attributes
	}
	before := attributeOrder[:slices.Index(attributeOrder, key)]
	position := 0
	for index, attr := range attributes {
		if slices.Contains(before, attr.key) {
			position = index + 1
		}
	}
	return slices.Insert(attributes, position, attribute{key: key, value: value})
}

// appendEntry appends an entry's lines in the file's line endings, after a
// blank line.
func appendEntry(data []byte, ticket string, lines []string) []byte {
	text := string(data)
	newline := "\n"
	if end := strings.Index(text, "\n"); end > 0 && text[end-1] == '\r' {
		newline = "\r\n"
	}
	var out strings.Builder
	out.WriteString(text)
	switch {
	case text == "":
		out.WriteString("# Activity: " + ticket + newline)
	case !strings.HasSuffix(text, "\n"):
		out.WriteString(newline)
	}
	out.WriteString(newline + strings.Join(lines, newline) + newline)
	return []byte(out.String())
}

// line is a change as its history bullet.
func (c Change) line() string {
	switch c.What {
	case ChangeCreated:
		if c.To == "" {
			return "- " + c.What
		}
		return "- " + c.What + ": " + c.To
	case ChangeMoved:
		return "- " + c.What + ": " + c.From + " → " + c.To
	case ChangeEdited:
		return "- " + c.What + ": " + strings.Join(c.Fields, ", ")
	case ChangeTicked, ChangeUnticked:
		var items []string
		for _, n := range c.Criteria {
			items = append(items, "criterion "+strconv.Itoa(n))
		}
		for _, n := range c.Steps {
			items = append(items, "review step "+strconv.Itoa(n))
		}
		return "- " + c.What + ": " + strings.Join(items, ", ")
	}
	return "- " + c.What
}
