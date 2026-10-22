package board

import (
	"regexp"
	"slices"
	"strings"
)

// Questions in a ticket's history (FH-190, board-format §Activity): an
// agent asked a question about the ticket, a person answered it on the
// board, the user answered it in the session's own chat, or its session
// ended with it still open. They are the one history that carries text,
// the question and the answer, since the decision is what is worth
// keeping. The text follows the bullet on indented `key: value` lines, so
// none of it can start a heading, an entry or a code fence.

// What a question change records.
const (
	ChangeAsked             = "asked"
	ChangeAnswered          = "answered"
	ChangeAnsweredInSession = "answered-in-session"
	ChangeSessionEnded      = "session-ended"
)

// Limits of a question's text in history, as the event log bounds them.
const (
	MaxQuestionText    = 1000
	MaxQuestionOptions = 10
	MaxOptionText      = 200
	MaxAnswerText      = 1000
)

// QuestionRecord is the question a question change is about: its id
// (agent-protocol §7), kind, text and options when asked, and the answer
// when answered on the board.
type QuestionRecord struct {
	ID      string
	Kind    string
	Text    string
	Options []string
	Answer  string
}

var (
	questionChanges = []string{ChangeAsked, ChangeAnswered, ChangeAnsweredInSession, ChangeSessionEnded}
	// questionLine is one line of a question change's text.
	questionLine = regexp.MustCompile(`^[ \t]+(kind|question|option|answer):(?: (.*))?$`)
)

// IsQuestionChange reports whether a change is about a question.
func IsQuestionChange(what string) bool { return slices.Contains(questionChanges, what) }

// HasQuestionChange reports whether the history records what for question
// id, so a question's outcome is written once.
func (a Activity) HasQuestionChange(id, what string) bool {
	for _, entry := range a.History {
		for _, change := range entry.Changes {
			if change.What == what && change.Question.ID == id {
				return true
			}
		}
	}
	return false
}

// questionLines is a question change's bullet and text lines, its text
// cut to the limits; none when its id cannot be written.
func questionLines(what string, q QuestionRecord) []string {
	if !commentIDPattern.MatchString(q.ID) {
		return nil
	}
	lines := []string{"- " + what + ": question " + q.ID}
	add := func(key, value string) {
		lines = append(lines, strings.TrimRight("  "+key+": "+value, " \t"))
	}
	if kind := OneLine(q.Kind, 40); historyName.MatchString(kind) {
		add("kind", kind)
	}
	if text := OneLine(q.Text, MaxQuestionText); text != "" {
		add("question", text)
	}
	options := 0
	for _, option := range q.Options {
		if option = OneLine(option, MaxOptionText); option != "" && options < MaxQuestionOptions {
			add("option", option)
			options++
		}
	}
	if answer := answerText(q.Answer); answer != "" {
		for _, line := range strings.Split(answer, "\n") {
			add("answer", line)
		}
	}
	return lines
}

// answerText is an answer as history keeps it: its lines without trailing
// space or blank lines at its ends, at most MaxAnswerText characters.
func answerText(answer string) string {
	answer = strings.ReplaceAll(strings.ReplaceAll(answer, "\r\n", "\n"), "\r", "\n")
	if runes := []rune(answer); len(runes) > MaxAnswerText {
		answer = string(runes[:MaxAnswerText])
	}
	lines := strings.Split(answer, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, " \t")
	}
	return strings.Join(trimBlank(lines), "\n")
}

// parseQuestionChange reads a question bullet's argument, `question <id>`.
func parseQuestionChange(change Change, argument string) (Change, bool) {
	id, ok := strings.CutPrefix(argument, "question ")
	if !ok || !commentIDPattern.MatchString(strings.TrimSpace(id)) {
		return change, false
	}
	change.Question.ID = strings.TrimSpace(id)
	return change, true
}

// addQuestionLine adds a text line to the question change it follows; ok
// is false for a line that is not one.
func addQuestionLine(change *Change, line string) bool {
	match := questionLine.FindStringSubmatch(line)
	if match == nil {
		return false
	}
	q := &change.Question
	switch match[1] {
	case "kind":
		q.Kind = strings.TrimSpace(match[2])
	case "question":
		q.Text = match[2]
	case "option":
		q.Options = append(q.Options, match[2])
	case "answer":
		// The first line is never blank (answerText).
		if q.Answer != "" {
			q.Answer += "\n"
		}
		q.Answer += match[2]
	}
	return true
}

// ValidEntryID reports whether id can name an entry, or a question in
// history: 1 to 64 letters, digits, `_` or `-`.
func ValidEntryID(id string) bool { return commentIDPattern.MatchString(id) }
