package board

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Column is a ticket's status (board-format v2).
type Column string

// Columns in workflow order.
const (
	Backlog    Column = "backlog"
	UpNext     Column = "up-next"
	InProgress Column = "in-progress"
	Review     Column = "review"
	Done       Column = "done"
)

// Columns lists every column in workflow order.
var Columns = []Column{Backlog, UpNext, InProgress, Review, Done}

// Satisfies reports whether a ticket in this column no longer blocks others.
func (c Column) Satisfies() bool { return c == Review || c == Done }

// Open reports whether tickets in this column can be blocked or picked up.
func (c Column) Open() bool { return c == Backlog || c == UpNext || c == InProgress }

// ParseColumn returns the column named by a status value.
func ParseColumn(value string) (Column, bool) {
	column := Column(value)
	return column, slices.Contains(Columns, column)
}

// TicketTypes are the known ticket types.
var TicketTypes = []string{"feature", "test", "bug", "refactor", "infra", "docs", "spike"}

// Priorities are the known Priorities, highest first.
var Priorities = []string{"high", "medium", "low"}

var (
	keyPattern    = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)
	idPattern     = regexp.MustCompile(`^([A-Z][A-Z0-9]{1,9})-([1-9][0-9]*)$`)
	folderPattern = regexp.MustCompile(`^([A-Z][A-Z0-9]{1,9}-[1-9][0-9]*)(?:-(.+))?$`)
)

// ValidKey reports whether key is a project key (KEY-1).
func ValidKey(key string) bool { return keyPattern.MatchString(key) }

// ParseID splits a ticket id such as FH-42 into its key and number.
func ParseID(id string) (string, int, bool) {
	match := idPattern.FindStringSubmatch(id)
	if match == nil {
		return "", 0, false
	}
	number, err := strconv.Atoi(match[2])
	if err != nil {
		return "", 0, false
	}
	return match[1], number, true
}

// ParseFolder splits a ticket folder name "<id>-<slug>" into id and slug.
func ParseFolder(folder string) (string, string, bool) {
	match := folderPattern.FindStringSubmatch(folder)
	if match == nil {
		return "", "", false
	}
	return match[1], match[2], true
}

// TicketFile picks a ticket folder's ticket file from the names in it: the
// file named after the folder (FH-42-slug/FH-42-slug.md), or else a single
// markdown file named after the folder's id, which happens when a folder is
// renamed by hand; that case returns a warning. ok is false when there is no
// ticket file or the choice is ambiguous.
func TicketFile(folder string, names []string) (file, warning string, ok bool) {
	want := folder + ".md"
	if slices.Contains(names, want) {
		return want, "", true
	}
	id, _, parsed := ParseFolder(folder)
	if !parsed {
		return "", "", false
	}
	var candidates []string
	for _, name := range names {
		if name == id+".md" || (strings.HasPrefix(name, id+"-") && strings.HasSuffix(name, ".md")) {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) != 1 {
		return "", "", false
	}
	return candidates[0], fmt.Sprintf("ticket file %s should be named %s", candidates[0], want), true
}

// DeriveKey proposes a project key from a directory name: the initials of a
// multi-word name, otherwise its first three letters, uppercased.
func DeriveKey(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	var key strings.Builder
	if len(words) > 1 {
		for _, word := range words {
			key.WriteRune(unicode.ToUpper([]rune(word)[0]))
			if key.Len() == 10 {
				break
			}
		}
	} else if len(words) == 1 {
		for _, r := range words[0] {
			if r > unicode.MaxASCII {
				continue
			}
			key.WriteRune(unicode.ToUpper(r))
			if key.Len() == 3 {
				break
			}
		}
	}
	result := key.String()
	if result == "" || !unicode.IsLetter(rune(result[0])) {
		result = "P" + result
		if len(words) == 1 && len(result) > 3 {
			result = result[:3]
		}
	}
	if len(result) < 2 {
		result += "X"
	}
	if len(result) > 10 {
		result = result[:10]
	}
	return result
}

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

// Ticket is one parsed ticket.
type Ticket struct {
	// ID is the ticket id (FH-42); Folder the ticket folder name; Slug the
	// folder's readable part.
	ID, Folder, Slug string
	Column           Column

	Title, Type, Created, Priority        string
	Session, GitRef, Branch, Workstream   string
	Updated                               string
	DependsOn, DependsOnWorkstreams, Tags []string

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

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ParseTicket parses a ticket file, tickets/<folder>/<folder>.md. It never fails: problems that
// stop the ticket being understood become repair reasons, the rest warnings.
func ParseTicket(folder string, data []byte) (ticket Ticket) {
	doc := mdfile.Parse(data)
	ticket = Ticket{Folder: folder, Column: Backlog, Body: doc.Body, FrontmatterRaw: doc.FrontmatterRaw}
	folderID, slug, folderOK := ParseFolder(folder)
	ticket.Slug = slug

	ticket.Title = mdfile.Title(doc.Body)
	ticket.Excerpt = excerpt(doc.Body)
	if section, ok := mdfile.FindSection(doc.Body, "Acceptance Criteria"); ok {
		for _, box := range mdfile.Checkboxes(section.Content) {
			ticket.Criteria = append(ticket.Criteria, Criterion{Text: box.Text, Done: box.Checked})
		}
	}
	if section, ok := mdfile.FindSection(doc.Body, "Handoff"); ok {
		ticket.Handoff = &Handoff{Markdown: strings.TrimSpace(section.Content), Next: handoffList(section.Content, "Next")}
	}
	defer func() {
		if ticket.Title == "" {
			ticket.Title = ticket.ID
		}
		if ticket.Title == "" {
			ticket.Title = folder
		}
	}()

	if doc.FrontmatterError != nil {
		ticket.Repair = append(ticket.Repair, doc.FrontmatterError.Error())
		if folderOK {
			ticket.ID = folderID
		}
		return ticket
	}
	if !doc.HasFrontmatter {
		ticket.Warnings = append(ticket.Warnings, "no frontmatter")
	}

	id, hasID := doc.String("id")
	switch {
	case hasID && id != "":
		if _, _, ok := ParseID(id); !ok {
			ticket.Repair = append(ticket.Repair, fmt.Sprintf("id %q is not a ticket id like FH-42", id))
		} else {
			ticket.ID = id
			if folderOK && folderID != id {
				ticket.Warnings = append(ticket.Warnings, fmt.Sprintf("id %s does not match the folder name %s", id, folder))
			}
		}
	case folderOK:
		ticket.ID = folderID
		ticket.Warnings = append(ticket.Warnings, fmt.Sprintf("missing id; using %s from the folder name", folderID))
	default:
		ticket.Repair = append(ticket.Repair, "no ticket id: add id: <KEY>-<number> to the frontmatter")
	}

	status, hasStatus := doc.String("status")
	if column, ok := ParseColumn(status); ok {
		ticket.Column = column
	} else if !hasStatus || status == "" {
		ticket.Warnings = append(ticket.Warnings, "missing status; shown in Backlog")
	} else {
		ticket.Repair = append(ticket.Repair, fmt.Sprintf("status %q is not one of %s", status, joinColumns()))
	}

	ticket.Type, _ = doc.String("type")
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
		{"type", ticket.Type}, {"priority", ticket.Priority}, {"created", ticket.Created},
	} {
		if field.value == "" {
			missing = append(missing, field.name)
		}
	}
	if len(missing) > 0 {
		ticket.Warnings = append(ticket.Warnings, "missing "+strings.Join(missing, ", "))
	}
	if ticket.Type != "" && !slices.Contains(TicketTypes, ticket.Type) {
		ticket.Warnings = append(ticket.Warnings, fmt.Sprintf("type %q is not a known ticket type", ticket.Type))
	}
	if ticket.Priority != "" && !slices.Contains(Priorities, ticket.Priority) {
		ticket.Warnings = append(ticket.Warnings, fmt.Sprintf("priority %q should be high, medium or low", ticket.Priority))
	}
	if ticket.Created != "" && !validDate(ticket.Created) {
		ticket.Warnings = append(ticket.Warnings, fmt.Sprintf("created %q should be YYYY-MM-DD", ticket.Created))
	}
	return ticket
}

func joinColumns() string {
	names := make([]string, len(Columns))
	for index, column := range Columns {
		names[index] = string(column)
	}
	return strings.Join(names, ", ")
}

func validDate(value string) bool {
	if !datePattern.MatchString(value) {
		return false
	}
	_, err := time.Parse(time.DateOnly, value)
	return err == nil
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

// Attachment is one entry of a ticket's files/index.yaml.
type Attachment struct {
	File, Caption, Kind, Source, Run, Added, SHA256 string
}

// Project is one project directory's parsed contents.
type Project struct {
	Name, DisplayName string
	// Key is the ticket id prefix (KEY-1); KeyDerived means project.yaml does
	// not set it yet.
	Key         string
	KeyDerived  bool
	NextID      int
	Repos       []string
	Tickets     []Ticket
	Workstreams []Workstream
	// Archived lists ids of archived tickets; they count as done.
	Archived []string
	// Reviews holds the ids that have a review file.
	Reviews map[string]bool
	// Attachments maps ticket ids to their files index.
	Attachments map[string][]Attachment
	// EnforceHandoff is project.yaml's settings.enforce_handoff; nil means
	// the global setting applies (HOOK-6).
	EnforceHandoff *bool
	// Warnings are problems with project-level files (project.yaml, keys).
	Warnings []string
	// LastModified is the newest modification time among the project's files.
	LastModified time.Time
}

// CheckProject marks tickets that share an id as needing repair and warns
// about ids whose key is not the project's.
func CheckProject(project *Project) {
	folders := map[string][]string{}
	for _, ticket := range project.Tickets {
		if ticket.ID != "" {
			folders[ticket.ID] = append(folders[ticket.ID], ticket.Folder)
		}
	}
	for index := range project.Tickets {
		ticket := &project.Tickets[index]
		if ticket.ID == "" {
			continue
		}
		if copies := folders[ticket.ID]; len(copies) > 1 {
			var others []string
			for _, folder := range copies {
				if folder != ticket.Folder {
					others = append(others, folder)
				}
			}
			ticket.Repair = append(ticket.Repair, fmt.Sprintf("id %s is also used by %s; give one a new id", ticket.ID, strings.Join(others, ", ")))
		}
		if key, _, ok := ParseID(ticket.ID); ok && project.Key != "" && key != project.Key {
			ticket.Warnings = append(ticket.Warnings, fmt.Sprintf("id %s uses key %s but this project's key is %s", ticket.ID, key, project.Key))
		}
	}
}

// InProgressOnBranch lists the in-progress tickets whose branch is branch,
// the provisional link of a run on that branch (RUN-5). Tickets that need
// repair are left out: their fields cannot be trusted.
func (p Project) InProgressOnBranch(branch string) []string {
	if branch == "" {
		return nil
	}
	var ids []string
	for _, ticket := range p.Tickets {
		if ticket.Column == InProgress && ticket.Branch == branch && !ticket.NeedsRepair() {
			ids = append(ids, ticket.ID)
		}
	}
	return ids
}

// OwnsIDs reports whether the project has tickets, live or archived, so its
// key is fixed (KEY-5).
func (p Project) OwnsIDs() bool { return len(p.Tickets) > 0 || len(p.Archived) > 0 }

// CheckKeys warns on projects that share a key, which makes ids ambiguous. A
// derived key on a project with no tickets is only a suggestion and is
// skipped.
func CheckKeys(b *Board) {
	owners := map[string][]string{}
	for _, project := range b.Projects {
		if project.KeyDerived && !project.OwnsIDs() {
			continue
		}
		owners[project.Key] = append(owners[project.Key], project.Name)
	}
	for index := range b.Projects {
		project := &b.Projects[index]
		if project.KeyDerived && !project.OwnsIDs() {
			continue
		}
		var others []string
		for _, name := range owners[project.Key] {
			if name != project.Name {
				others = append(others, name)
			}
		}
		if len(others) > 0 {
			project.Warnings = append(project.Warnings, fmt.Sprintf("key %s is also used by %s; set a unique key in project.yaml", project.Key, strings.Join(others, ", ")))
		}
	}
}
