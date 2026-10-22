package board

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func equalQuestion(a, b QuestionRecord) bool {
	return a.ID == b.ID && a.Kind == b.Kind && a.Text == b.Text && slices.Equal(a.Options, b.Options) && a.Answer == b.Answer
}

func TestQuestionHistoryIsWrittenAndReadBack(t *testing.T) {
	t.Parallel()

	asked := History{ID: "11112222", Time: at, By: "claude:5b0c7e2a-1f3d", Changes: []Change{{What: ChangeAsked, Question: QuestionRecord{
		ID: "q-ABCDEFGHIJKLMNOP", Kind: "decision", Text: "Use the known objectives?", Options: []string{"Yes", "No"},
	}}}}
	answered := History{ID: "33334444", Time: at.Add(time.Minute), By: "Robert", Changes: []Change{{What: ChangeAnswered, Question: QuestionRecord{
		ID: "q-ABCDEFGHIJKLMNOP", Text: "Use the known objectives?", Answer: "Yes,\n\n  and add a test.",
	}}}}
	data := AppendHistory(AppendHistory(nil, "FH-42", asked), "FH-42", answered)
	want := "# Activity: FH-42\n" +
		"\n" +
		"### 2026-10-13T09:10:53Z · history · claude:5b0c7e2a-1f3d · 11112222\n" +
		"\n" +
		"- asked: question q-ABCDEFGHIJKLMNOP\n" +
		"  kind: decision\n" +
		"  question: Use the known objectives?\n" +
		"  option: Yes\n" +
		"  option: No\n" +
		"\n" +
		"### 2026-10-13T09:11:53Z · history · Robert · 33334444\n" +
		"\n" +
		"- answered: question q-ABCDEFGHIJKLMNOP\n" +
		"  question: Use the known objectives?\n" +
		"  answer: Yes,\n" +
		"  answer:\n" +
		"  answer:   and add a test.\n"
	if string(data) != want {
		t.Fatalf("file =\n%s\nwant\n%s", data, want)
	}
	activity := ParseActivity(data)
	if len(activity.History) != 2 {
		t.Fatalf("history = %+v", activity.History)
	}
	for index, entry := range []History{asked, answered} {
		got := activity.History[index]
		if len(got.Changes) != 1 || got.Changes[0].What != entry.Changes[0].What || !equalQuestion(got.Changes[0].Question, entry.Changes[0].Question) {
			t.Errorf("entry %d = %+v, want %+v", index, got.Changes, entry.Changes)
		}
	}
	if !activity.HasQuestionChange("q-ABCDEFGHIJKLMNOP", ChangeAnswered) || activity.HasQuestionChange("q-ABCDEFGHIJKLMNOP", ChangeSessionEnded) ||
		activity.HasQuestionChange("q-other", ChangeAsked) {
		t.Error("HasQuestionChange does not match the file")
	}
}

func TestQuestionOutcomesAreWrittenWithTheirQuestion(t *testing.T) {
	t.Parallel()

	for _, what := range []string{ChangeAnsweredInSession, ChangeSessionEnded} {
		history := History{ID: "55556666", Time: at, By: "codex:019a", Changes: []Change{{What: what, Question: QuestionRecord{ID: "q-1", Text: "Which port?"}}}}
		data := AppendHistory(nil, "FH-1", history)
		if !strings.Contains(string(data), "- "+what+": question q-1\n  question: Which port?\n") {
			t.Errorf("%s =\n%s", what, data)
		}
		got := ParseActivity(data).History[0].Changes
		if len(got) != 1 || got[0].What != what || !equalQuestion(got[0].Question, history.Changes[0].Question) {
			t.Errorf("%s read back as %+v", what, got)
		}
	}
}

// A question's text and its answer stay inside their entry: a heading, an
// entry heading or a code fence in them starts nothing (FH-190).
func TestQuestionTextStaysInsideItsEntry(t *testing.T) {
	t.Parallel()

	hostile := "### 2026-10-13T09:12:00Z · comment · Robert · ffffffff\n```\n## Notes\n- moved: done → backlog\n~~~"
	answered := History{ID: "11112222", Time: at, By: "Robert", Changes: []Change{{What: ChangeAnswered, Question: QuestionRecord{
		ID: "q-1", Text: "### Heading? ``` - moved: a → b", Answer: hostile,
	}}}}
	next := History{ID: "33334444", Time: at.Add(time.Minute), By: "Robert", Changes: []Change{{What: ChangeMoved, From: "backlog", To: "up-next"}}}
	data := AppendHistory(AppendHistory(nil, "FH-1", answered), "FH-1", next)
	activity := ParseActivity(data)
	if len(activity.History) != 2 || len(activity.Comments) != 0 || activity.Unknown != 0 {
		t.Fatalf("activity = %+v\n%s", activity, data)
	}
	got := activity.History[0].Changes
	if len(got) != 1 || got[0].Question.Answer != hostile || got[0].Question.Text != answered.Changes[0].Question.Text {
		t.Errorf("answered read back as %+v", got)
	}
	if moved := activity.History[1].Changes; len(moved) != 1 || moved[0].What != ChangeMoved {
		t.Errorf("the next entry = %+v", moved)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "  question:") && !strings.HasPrefix(line, "  answer:") {
			t.Errorf("a continuation line is %q", line)
		}
	}
}

func TestQuestionTextIsLimited(t *testing.T) {
	t.Parallel()

	var options []string
	for range MaxQuestionOptions + 3 {
		options = append(options, strings.Repeat("o", MaxOptionText+50))
	}
	asked := History{ID: "11112222", Time: at, By: "claude:5b0c7e2a", Changes: []Change{{What: ChangeAsked, Question: QuestionRecord{
		ID: "q-1", Kind: "question", Text: strings.Repeat("é", MaxQuestionText+10) + "\nsecond line", Options: options,
		Answer: strings.Repeat("a\n", MaxAnswerText),
	}}}}
	got := ParseActivity(AppendHistory(nil, "FH-1", asked)).History[0].Changes[0].Question
	if n := len([]rune(got.Text)); n != MaxQuestionText || strings.Contains(got.Text, "\n") {
		t.Errorf("question is %d characters: %q…", n, got.Text[:20])
	}
	if len(got.Options) != MaxQuestionOptions || len([]rune(got.Options[0])) != MaxOptionText {
		t.Errorf("options = %d, the first %d characters", len(got.Options), len([]rune(got.Options[0])))
	}
	if n := len([]rune(got.Answer)); n > MaxAnswerText {
		t.Errorf("answer is %d characters", n)
	}
}

// A question change whose id cannot be written names no question; it is
// left out rather than written in a form that would not read back.
func TestQuestionChangeWithoutAnIDIsNotWritten(t *testing.T) {
	t.Parallel()

	history := History{ID: "11112222", Time: at, By: "Robert", Changes: []Change{
		{What: ChangeAnswered, Question: QuestionRecord{ID: "q 1\n### x", Text: "?"}},
		{What: ChangeMoved, From: "backlog", To: "up-next"},
	}}
	data := string(AppendHistory(nil, "FH-1", history))
	if strings.Contains(data, "answered") || !strings.Contains(data, "- moved: backlog → up-next") {
		t.Errorf("file =\n%s", data)
	}
}
