package api

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
)

// The vocabularies the browser keeps its own copies of (FH-97, D79): the
// server's lists, written to testdata/vocabulary.json, which the
// frontend's tests (frontend/src/api/vocabulary.test.ts) match its copies
// to. Rewrite it with go test ./internal/api -run TestVocabulary -update.

const vocabularyFile = "../../testdata/vocabulary.json"

// Vocabulary is testdata/vocabulary.json.
type Vocabulary struct {
	TicketTypes   []string            `json:"ticketTypes"`
	Priorities    []string            `json:"priorities"`
	Columns       []string            `json:"columns"`
	RunStates     []string            `json:"runStates"`
	QuestionKinds []string            `json:"questionKinds"`
	Preferences   map[string][]string `json:"preferences"`
	// TicketIDs are sample ids and whether the board reads each as one.
	TicketIDs  map[string]bool `json:"ticketIds"`
	ErrorCodes []string        `json:"errorCodes"`
}

func vocabulary() Vocabulary {
	v := Vocabulary{
		TicketTypes: board.TicketTypes, Priorities: board.Priorities,
		RunStates:     []string{string(runs.NeedsYou), string(runs.Working), string(runs.Quiet), string(runs.Waiting), string(runs.Ended)},
		QuestionKinds: events.QuestionKinds,
		Preferences:   config.Choices(),
		TicketIDs:     map[string]bool{},
		ErrorCodes:    ErrorCodes,
	}
	for _, column := range board.Columns {
		v.Columns = append(v.Columns, string(column))
	}
	for _, id := range []string{"FH-1", "FH-42", "AB-100", "A1-7", "ABCDEFGHIJ-3", "ABCDEFGHIJK-3", "F-1", "FH-0", "FH-01", "fh-1", "FH1", "1H-1", "FH-1x", "FH--1"} {
		_, _, ok := board.ParseID(id)
		v.TicketIDs[id] = ok
	}
	return v
}

func TestVocabulary(t *testing.T) {
	t.Parallel()

	var want bytes.Buffer
	encoder := json.NewEncoder(&want)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(vocabulary()); err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(vocabularyFile, want.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(vocabularyFile)
	if err != nil || !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("testdata/vocabulary.json is stale (%v); rewrite it with go test ./internal/api -run TestVocabulary -update, then make the frontend agree\n%s", err, want.Bytes())
	}
}

// Every error code a handler writes is one of ErrorCodes, written as a
// literal, and every listed code is written somewhere.
func TestErrorCodes(t *testing.T) {
	t.Parallel()

	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	call := regexp.MustCompile(`writeError\(`)
	literal := regexp.MustCompile(`writeError\(\s*w,\s*[^,]+,\s*"([a-z_]+)"|Code:\s*"([a-z_]+)"`)
	used := map[string]bool{}
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		calls := len(call.FindAll(data, -1))
		matches := literal.FindAllSubmatch(data, -1)
		literals := 0
		for _, match := range matches {
			code := string(match[1]) + string(match[2])
			used[code] = true
			if len(match[1]) > 0 {
				literals++
			}
			if !slices.Contains(ErrorCodes, code) {
				t.Errorf("%s writes error code %q, which ErrorCodes does not list", source, code)
			}
		}
		// writeError's own definition is the one call without a literal.
		if source == "api.go" {
			calls--
		}
		if calls != literals {
			t.Errorf("%s: %d writeError calls, %d with a literal code", source, calls, literals)
		}
	}
	for _, code := range ErrorCodes {
		if !used[code] {
			t.Errorf("ErrorCodes lists %q, which no handler writes", code)
		}
	}
}
