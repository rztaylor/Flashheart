package board

import (
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// History is derived from what a write changed, not from what its caller
// says it did, so every ticket write is described alike (board-format
// §Activity): the column it moved between, the names of the fields and
// sections it edited, the numbers of the criteria it ticked. It never
// carries the ticket's text: a key or section heading Flashheart does not
// know is named only as "frontmatter" or "body".

var (
	// historyName is a frontmatter key or status history names as it is.
	historyName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,39}$`)
	// taskBox is a criterion's box, so ticks can be told from edits.
	taskBox = regexp.MustCompile(`(?m)^([-*+] )\[[ xX]\]`)
)

// historyQuiet are the frontmatter keys not reported as edited: status, a
// move of its own, the stamp of every write, and the manual order (EDIT-9).
var historyQuiet = []string{"status", "updated", "rank"}

// historyFields names the sections history reports by name; the notes and
// the handoff have their own place in the ticket's activity.
var historyFields = map[string]string{
	strings.ToLower(SectionDescription):  "description",
	strings.ToLower(SectionCriteria):     "acceptance-criteria",
	strings.ToLower(SectionTestPlan):     "test-plan",
	strings.ToLower(SectionReproduction): "reproduction",
	strings.ToLower(SectionContext):      "context",
}

var historySkipped = []string{strings.ToLower(SectionNotes), strings.ToLower(SectionHandoff)}

// TicketChanges is what a write changed in a ticket file, as history
// records it: moved, then edited, ticked and unticked; nil when only the
// stamp, the rank, the notes or the handoff changed.
func TicketChanges(before, after []byte) []Change {
	old, current := mdfile.Parse(before), mdfile.Parse(after)
	var changes []Change
	var fields []string
	if old.FrontmatterError == nil && current.FrontmatterError == nil {
		from, _ := old.String("status")
		to, _ := current.String("status")
		switch {
		case from == to:
		case historyName.MatchString(from) && historyName.MatchString(to):
			changes = append(changes, Change{What: ChangeMoved, From: from, To: to})
		default:
			fields = append(fields, "status")
		}
		fields = append(fields, frontmatterChanges(old, current)...)
	} else if old.FrontmatterRaw != current.FrontmatterRaw || (old.FrontmatterError == nil) != (current.FrontmatterError == nil) {
		fields = append(fields, "frontmatter")
	}
	if mdfile.Title(old.Body) != mdfile.Title(current.Body) {
		fields = append(fields, "title")
	}
	edited, ticked, unticked := bodyChanges(old.Body, current.Body)
	fields = append(fields, edited...)
	if len(fields) > 0 {
		changes = append(changes, Change{What: ChangeEdited, Fields: dedupe(fields)})
	}
	if len(ticked) > 0 {
		changes = append(changes, Change{What: ChangeTicked, Criteria: ticked})
	}
	if len(unticked) > 0 {
		changes = append(changes, Change{What: ChangeUnticked, Criteria: unticked})
	}
	return changes
}

// frontmatterChanges names the keys whose values differ, in the new file's
// order and then the removed ones.
func frontmatterChanges(old, current mdfile.Document) []string {
	value := func(doc mdfile.Document, key string) any {
		var v any
		_ = doc.Decode(key, &v)
		return v
	}
	var fields []string
	seen := map[string]bool{}
	for _, list := range [][]mdfile.Field{current.Fields(), old.Fields()} {
		for _, field := range list {
			if seen[field.Key] || slices.Contains(historyQuiet, field.Key) {
				continue
			}
			seen[field.Key] = true
			if !reflect.DeepEqual(value(old, field.Key), value(current, field.Key)) {
				fields = append(fields, fieldName(field.Key))
			}
		}
	}
	return fields
}

// fieldName is a key as history names it.
func fieldName(key string) string {
	if historyName.MatchString(key) {
		return key
	}
	return "frontmatter"
}

// bodyChanges names the sections whose text changed and the criteria
// ticked and unticked when only their boxes changed.
func bodyChanges(old, current string) (edited []string, ticked, unticked []int) {
	before, after := sectionTexts(old), sectionTexts(current)
	keys := slices.Clone(after.order)
	for _, key := range before.order {
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		a, b := before.text[key], after.text[key]
		if a == b || slices.Contains(historySkipped, key) {
			continue
		}
		name, known := historyFields[key]
		if !known {
			name = "body"
		}
		if key == strings.ToLower(SectionCriteria) && taskBox.ReplaceAllString(a, "$1[ ]") == taskBox.ReplaceAllString(b, "$1[ ]") {
			was, now := mdfile.Checkboxes(a), mdfile.Checkboxes(b)
			for index := range min(len(was), len(now)) {
				switch {
				case !was[index].Checked && now[index].Checked:
					ticked = append(ticked, index+1)
				case was[index].Checked && !now[index].Checked:
					unticked = append(unticked, index+1)
				}
			}
			continue
		}
		edited = append(edited, name)
	}
	if before.preamble != after.preamble {
		edited = append(edited, "body")
	}
	return edited, ticked, unticked
}

type sections struct {
	order    []string
	text     map[string]string
	preamble string
}

// sectionTexts is a body's sections by lower-cased heading (repeated
// headings joined), and the text before them without the title.
func sectionTexts(body string) sections {
	result := sections{text: map[string]string{}}
	list := mdfile.Sections(body)
	lines := strings.Split(body, "\n")
	end := len(lines)
	if len(list) > 0 {
		end = list[0].Line - 1
	}
	// The title is compared on its own (TicketChanges).
	preamble := strings.Join(lines[:end], "\n")
	result.preamble = strings.TrimSpace(strings.Replace(preamble, mdfile.Title(body), "", 1))
	for _, section := range list {
		key := strings.ToLower(strings.TrimSpace(section.Heading))
		if _, ok := result.text[key]; !ok {
			result.order = append(result.order, key)
		}
		result.text[key] += strings.TrimSpace(section.Content)
	}
	return result
}

func dedupe(items []string) []string {
	var out []string
	for _, item := range items {
		if !slices.Contains(out, item) {
			out = append(out, item)
		}
	}
	return out
}

// ReviewChanges is what a write changed in a ticket's review.md, as
// history records it: the How to Verify steps ticked and unticked when
// only their boxes changed (REV-3), otherwise that the review was edited.
func ReviewChanges(before, after string) []Change {
	if before == after {
		return nil
	}
	old, current := ParseReview(before), ParseReview(after)
	same := old.Before == current.Before && old.Intro == current.Intro && old.Outro == current.Outro &&
		old.After == current.After && len(old.Steps) == len(current.Steps)
	var ticked, unticked []int
	for index := range old.Steps {
		if !same {
			break
		}
		was, now := old.Steps[index], current.Steps[index]
		switch {
		case was.Text != now.Text:
			same = false
		case !was.Done && now.Done:
			ticked = append(ticked, index+1)
		case was.Done && !now.Done:
			unticked = append(unticked, index+1)
		}
	}
	if !same || len(ticked)+len(unticked) == 0 {
		return []Change{{What: ChangeEdited, Fields: []string{"review"}}}
	}
	var changes []Change
	if len(ticked) > 0 {
		changes = append(changes, Change{What: ChangeTicked, Steps: ticked})
	}
	if len(unticked) > 0 {
		changes = append(changes, Change{What: ChangeUnticked, Steps: unticked})
	}
	return changes
}
