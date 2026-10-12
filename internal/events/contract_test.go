package events

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const secret = "AKIAIOSFODNN7EXAMPLE"

// fill sets every string in v, however deep, to s.
func fill(v reflect.Value, s string) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				fill(v.Field(i), s)
			}
		}
	case reflect.Slice:
		item := reflect.New(v.Type().Elem()).Elem()
		fill(item, s)
		v.Set(reflect.Append(reflect.MakeSlice(v.Type(), 0, 1), item))
	}
}

// Every data type's agent strings are scrubbed by Append alone, whoever
// built the event (FH-92, SEC-3).
func TestAppendRedactsEveryStringOfEveryDataType(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	for kind, dataType := range kinds {
		data := reflect.New(dataType).Elem()
		fill(data, "note "+secret+" end")
		if err := log.Append(Event{Time: t0, Run: "claude:abc", Agent: "claude", Kind: kind, Project: "alpha", Data: data.Interface()}); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	written, err := os.ReadFile(filepath.Join(root, "alpha", ".flashheart", "events", FileName(t0)))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(written)), "\n")
	if len(lines) != len(kinds) {
		t.Fatalf("%d lines for %d kinds", len(lines), len(kinds))
	}
	for _, line := range lines {
		if strings.Contains(line, secret) {
			t.Errorf("secret kept: %s", line)
		}
	}
}

// The data must be the kind's own type: a mismatch is refused, and nothing
// of the batch is written (FH-92).
func TestAppendRefusesDataOfAnotherKind(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	good := Event{Time: t0, Run: "claude:abc", Agent: "claude", Kind: TurnEnd, Project: "alpha", Data: TurnEndData{}}
	for _, bad := range []Event{
		{Time: t0, Run: "claude:abc", Agent: "claude", Kind: Claim, Project: "alpha", Data: CheckpointData{Ticket: "FH-1"}},
		{Time: t0, Run: "claude:abc", Agent: "claude", Kind: TurnStart, Project: "alpha", Data: &TurnStartData{}},
		{Time: t0, Run: "claude:abc", Agent: "claude", Kind: "made.up", Project: "alpha", Data: TurnEndData{}},
	} {
		if err := log.Append(good, bad); err == nil {
			t.Errorf("%s with %T was appended", bad.Kind, bad.Data)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", ".flashheart", "events", FileName(t0))); !os.IsNotExist(err) {
		t.Fatalf("a refused batch wrote: %v", err)
	}
}

// Bounds hold whoever built the event: long lists and texts are cut.
func TestAppendBoundsAgentStrings(t *testing.T) {
	t.Parallel()

	options := make([]string, MaxOptions+5)
	for i := range options {
		options[i] = strings.Repeat("o", MaxOptionText+50)
	}
	data := QuestionData{ID: "q-1", Kind: QuestionKindDecision, Text: strings.Repeat("t", MaxQuestionText+10), Options: options}
	bounded := data.bound().(QuestionData)
	if len(bounded.Options) != MaxOptions || len([]rune(bounded.Options[0])) != MaxOptionText || len([]rune(bounded.Text)) != MaxQuestionText {
		t.Fatalf("bounded = %d options, option %d runes, text %d runes", len(bounded.Options), len([]rune(bounded.Options[0])), len([]rune(bounded.Text)))
	}
	// A human's answer keeps its lines.
	if answer := (AnswerData{Answer: "Yes.\nBut later."}).bound().(AnswerData); answer.Answer != "Yes.\nBut later." {
		t.Fatalf("answer = %q", answer.Answer)
	}
	if !reflect.DeepEqual(TurnEndData{BlockedForHandoff: true}.bound(), TurnEndData{BlockedForHandoff: true}) {
		t.Fatal("a type with no strings changed")
	}
}
