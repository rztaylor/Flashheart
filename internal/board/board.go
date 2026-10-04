package board

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Column is a ticket status directory.
type Column string

// Columns in workflow order.
const (
	Todo          Column = "todo"
	InProgress    Column = "in-progress"
	ReadyToReview Column = "ready-to-review"
	Done          Column = "done"
)

// Columns lists every real column in workflow order.
var Columns = []Column{Todo, InProgress, ReadyToReview, Done}

// Satisfies reports whether a ticket in this column no longer blocks others.
func (c Column) Satisfies() bool { return c == ReadyToReview || c == Done }

// typeByPrefix maps filename prefixes to frontmatter types.
var typeByPrefix = map[string]string{
	"feat": "feature", "test": "test", "bug": "bug", "refactor": "refactor",
	"infra": "infra", "docs": "docs", "spike": "spike",
}

var priorities = []string{"high", "medium", "low"}

// ExcerptRunes bounds Ticket.Excerpt.
const ExcerptRunes = 200

// Criterion is one acceptance-criteria checkbox.
type Criterion struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// Handoff is a ticket's ## Handoff section.
type Handoff struct {
	Markdown string
	Next     []string
}

// Ticket is one parsed ticket file.
type Ticket struct {
	Slug   string
	Column Column

	Title, Type, Project, Created, Priority string
	Session, GitRef, Branch, Workstream     string
	Updated                                 string
	DependsOn, DependsOnWorkstreams, Tags   []string

	Criteria []Criterion
	Handoff  *Handoff
	Excerpt  string

	Body           string
	FrontmatterRaw string

	// Repair lists reasons the ticket needs repair (STO-4); Warnings are
	// format problems that do not stop it being read.
	Repair   []string
	Warnings []string

	// Modified is the file's modification time, set by the reader.
	Modified time.Time
}

// NeedsRepair reports whether the ticket must be shown as needing repair.
func (t Ticket) NeedsRepair() bool { return len(t.Repair) > 0 }

// Prefix returns the filename type prefix ("feat" for feat--x), or "".
func (t Ticket) Prefix() string {
	prefix, _, found := strings.Cut(t.Slug, "--")
	if !found {
		return ""
	}
	return prefix
}

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ParseTicket parses a ticket file. It never fails: unparseable frontmatter
// becomes a repair reason and format problems become warnings.
func ParseTicket(slug string, column Column, data []byte) Ticket {
	doc := mdfile.Parse(data)
	ticket := Ticket{Slug: slug, Column: column, Body: doc.Body, FrontmatterRaw: doc.FrontmatterRaw}
	prefixType, knownPrefix := typeByPrefix[ticket.Prefix()]
	if !knownPrefix {
		ticket.Warnings = append(ticket.Warnings, "filename should be <type>--<slug>.md with a type prefix such as feat--")
	}

	ticket.Title = mdfile.Title(doc.Body)
	if ticket.Title == "" {
		ticket.Title = slug
	}
	ticket.Excerpt = excerpt(doc.Body)
	if section, ok := mdfile.FindSection(doc.Body, "Acceptance Criteria"); ok {
		for _, box := range mdfile.Checkboxes(section.Content) {
			ticket.Criteria = append(ticket.Criteria, Criterion{Text: box.Text, Done: box.Checked})
		}
	}
	if section, ok := mdfile.FindSection(doc.Body, "Handoff"); ok {
		ticket.Handoff = &Handoff{Markdown: strings.TrimSpace(section.Content), Next: handoffList(section.Content, "Next")}
	}

	if doc.FrontmatterError != nil {
		ticket.Repair = append(ticket.Repair, doc.FrontmatterError.Error())
		ticket.Type = prefixType
		return ticket
	}
	if !doc.HasFrontmatter {
		ticket.Warnings = append(ticket.Warnings, "no frontmatter")
	}

	ticket.Type, _ = doc.String("type")
	ticket.Project, _ = doc.String("project")
	ticket.Created, _ = doc.String("created")
	ticket.Priority, _ = doc.String("priority")
	ticket.Session, _ = doc.String("session")
	ticket.GitRef, _ = doc.String("git-ref")
	ticket.Branch, _ = doc.String("branch")
	ticket.Workstream, _ = doc.String("workstream")
	ticket.Updated, _ = doc.String("updated")
	ticket.DependsOn = doc.List("depends-on")
	ticket.DependsOnWorkstreams = doc.List("depends-on-workstreams")
	ticket.Tags = doc.List("tags")

	var missing []string
	for _, field := range []struct{ name, value string }{
		{"type", ticket.Type}, {"project", ticket.Project}, {"created", ticket.Created}, {"priority", ticket.Priority},
	} {
		if field.value == "" {
			missing = append(missing, field.name)
		}
	}
	if len(missing) > 0 {
		ticket.Warnings = append(ticket.Warnings, "missing "+strings.Join(missing, ", "))
	}
	if ticket.Type != "" {
		if !slices.Contains(sortedTypes(), ticket.Type) {
			ticket.Warnings = append(ticket.Warnings, fmt.Sprintf("type %q is not a known ticket type", ticket.Type))
		} else if knownPrefix && ticket.Type != prefixType {
			ticket.Warnings = append(ticket.Warnings, fmt.Sprintf("type %q does not match the %s-- prefix", ticket.Type, ticket.Prefix()))
		}
	} else {
		ticket.Type = prefixType
	}
	if ticket.Priority != "" && !slices.Contains(priorities, ticket.Priority) {
		ticket.Warnings = append(ticket.Warnings, fmt.Sprintf("priority %q should be high, medium or low", ticket.Priority))
	}
	if ticket.Created != "" && !validDate(ticket.Created) {
		ticket.Warnings = append(ticket.Warnings, fmt.Sprintf("created %q should be YYYY-MM-DD", ticket.Created))
	}
	return ticket
}

func validDate(value string) bool {
	if !datePattern.MatchString(value) {
		return false
	}
	_, err := time.Parse(time.DateOnly, value)
	return err == nil
}

func sortedTypes() []string {
	types := make([]string, 0, len(typeByPrefix))
	for _, value := range typeByPrefix {
		types = append(types, value)
	}
	slices.Sort(types)
	return types
}

var (
	markdownLink  = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	markdownMarks = regexp.MustCompile("[*_`]+")
	whitespace    = regexp.MustCompile(`\s+`)
)

// excerpt returns the first paragraph of ## Description (or of the body
// after the title) as bounded plain text.
func excerpt(body string) string {
	source := body
	if section, ok := mdfile.FindSection(body, "Description"); ok {
		source = section.Content
	} else if sections := mdfile.Sections(body); len(sections) > 0 {
		source = body[:strings.Index(body, "## "+sections[0].Heading)]
	}
	var paragraph []string
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || (trimmed == "" && len(paragraph) == 0) {
			continue
		}
		if trimmed == "" {
			break
		}
		paragraph = append(paragraph, trimmed)
	}
	text := strings.Join(paragraph, " ")
	text = markdownLink.ReplaceAllString(text, "$1")
	text = markdownMarks.ReplaceAllString(text, "")
	text = strings.TrimSpace(whitespace.ReplaceAllString(text, " "))
	if runes := []rune(text); len(runes) > ExcerptRunes {
		text = strings.TrimSpace(string(runes[:ExcerptRunes])) + "…"
	}
	return text
}

var boldLabel = regexp.MustCompile(`^\*\*([^*]+)\*\*\s*$`)

// handoffList returns the bullet items under a **Label** line in a handoff.
func handoffList(content, label string) []string {
	var items []string
	inList := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if match := boldLabel.FindStringSubmatch(trimmed); match != nil {
			inList = strings.EqualFold(strings.TrimSpace(match[1]), label)
			continue
		}
		if inList && (strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ")) {
			items = append(items, strings.TrimSpace(trimmed[2:]))
		}
	}
	return items
}

// Workstream is one parsed workstream file.
type Workstream struct {
	Slug, Title, Status, Priority, Created string
	Tickets, DependsOnWorkstreams, Tags    []string
	Body                                   string
	Repair                                 []string
	Warnings                               []string
}

// NeedsRepair reports whether the workstream file must be repaired.
func (w Workstream) NeedsRepair() bool { return len(w.Repair) > 0 }

// ParseWorkstream parses a workstream file. It never fails.
func ParseWorkstream(slug string, data []byte) Workstream {
	doc := mdfile.Parse(data)
	workstream := Workstream{Slug: slug, Body: doc.Body, Title: mdfile.Title(doc.Body)}
	if workstream.Title == "" {
		workstream.Title = slug
	}
	if doc.FrontmatterError != nil {
		workstream.Repair = append(workstream.Repair, doc.FrontmatterError.Error())
		return workstream
	}
	workstream.Status, _ = doc.String("status")
	workstream.Priority, _ = doc.String("priority")
	workstream.Created, _ = doc.String("created")
	workstream.Tickets = doc.List("tickets")
	workstream.DependsOnWorkstreams = doc.List("depends-on-workstreams")
	workstream.Tags = doc.List("tags")
	if declared, ok := doc.String("slug"); ok && declared != "" && declared != slug {
		workstream.Warnings = append(workstream.Warnings, fmt.Sprintf("slug %q does not match the filename %s.md", declared, slug))
	}
	return workstream
}

// Attachment is one entry of an attachments index.yaml.
type Attachment struct {
	File, Caption, Kind, Run, Added string
}

// Project is one project directory's parsed contents.
type Project struct {
	Name, DisplayName string
	Repos             []string
	Tickets           []Ticket
	Workstreams       []Workstream
	// Archived lists slugs of archived tickets; they count as done.
	Archived []string
	// Reviews holds the slugs that have a review file.
	Reviews map[string]bool
	// Attachments maps ticket slugs to their attachment index.
	Attachments map[string][]Attachment
	// Warnings are problems with project-level files (project.yaml, indexes).
	Warnings []string
	// LastModified is the newest modification time among the project's files.
	LastModified time.Time
}

// MarkDuplicates marks every ticket whose slug exists in more than one
// column as needing repair.
func MarkDuplicates(project *Project) {
	columns := map[string][]Column{}
	for _, ticket := range project.Tickets {
		columns[ticket.Slug] = append(columns[ticket.Slug], ticket.Column)
	}
	for index := range project.Tickets {
		ticket := &project.Tickets[index]
		var others []string
		for _, column := range columns[ticket.Slug] {
			if column != ticket.Column {
				others = append(others, string(column))
			}
		}
		if len(columns[ticket.Slug]) > 1 {
			if len(others) == 0 {
				others = []string{string(ticket.Column)}
			}
			ticket.Repair = append(ticket.Repair, "the same ticket is also in "+strings.Join(others, ", ")+"; keep one copy")
		}
	}
}
