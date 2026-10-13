package board

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

var at = time.Date(2026, 10, 13, 9, 10, 53, 0, time.UTC)

// A file as Flashheart writes it, with an entry of a kind this version does
// not read and a hand-made heading inside a fence.
const activitySample = "# Activity: FH-42\n" +
	"\n" +
	"### 2026-10-13T09:10:53Z · history · Robert · 0a1b2c3d\n" +
	"\n" +
	"- created: backlog\n" +
	"\n" +
	"### 2026-10-13T09:12:00Z · comment · Robert · 4e5f6a7b\n" +
	"\n" +
	"state: change-requested\n" +
	"edited: 2026-10-13T09:20:00Z\n" +
	"quote: Acceptance Criteria 2\n" +
	"colour: blue\n" +
	"\n" +
	"> Tickets keep their place\n" +
	"> after a reload.\n" +
	"\n" +
	"They do not keep it in Done.\n" +
	"\n" +
	"```\n" +
	"### 2026-10-13T09:12:00Z · comment · Robert · ffffffff\n" +
	"```\n" +
	"\n" +
	"### 2026-10-13T09:13:00Z · reaction · Robert · 8c9d0e1f\n" +
	"\n" +
	"👍\n" +
	"\n" +
	"### 2026-10-13T09:14:00Z · history · claude:5b0c7e2a-1f3d · 2a3b4c5d\n" +
	"\n" +
	"- moved: up-next → in-progress\n" +
	"- edited: branch, title, description\n" +
	"- ticked: criterion 2, review step 1\n" +
	"- unticked: criterion 1\n" +
	"- reacted: twice\n"

func TestParseActivityReadsCommentsHistoryAndKeepsUnknownEntries(t *testing.T) {
	t.Parallel()

	activity := ParseActivity([]byte(activitySample))
	if len(activity.Comments) != 1 || len(activity.History) != 2 || activity.Unknown != 1 {
		t.Fatalf("activity = %+v", activity)
	}
	comment := activity.Comments[0]
	want := Comment{
		ID: "4e5f6a7b", Time: at.Add(67 * time.Second), By: "Robert", State: CommentChangeRequested,
		Edited: time.Date(2026, 10, 13, 9, 20, 0, 0, time.UTC), QuoteFrom: "Acceptance Criteria 2",
		Quote: "Tickets keep their place\nafter a reload.",
		Body:  "They do not keep it in Done.\n\n```\n### 2026-10-13T09:12:00Z · comment · Robert · ffffffff\n```",
	}
	if comment.ID != want.ID || !comment.Time.Equal(want.Time) || comment.By != want.By || comment.State != want.State ||
		!comment.Edited.Equal(want.Edited) || comment.QuoteFrom != want.QuoteFrom || comment.Quote != want.Quote || comment.Body != want.Body {
		t.Errorf("comment = %+v\nwant      %+v", comment, want)
	}
	created := activity.History[0]
	if created.ID != "0a1b2c3d" || !created.Time.Equal(at) || created.By != "Robert" || !slices.EqualFunc(created.Changes, []Change{{What: ChangeCreated, To: "backlog"}}, equalChange) {
		t.Errorf("created = %+v", created)
	}
	agent := activity.History[1]
	wantChanges := []Change{
		{What: ChangeMoved, From: "up-next", To: "in-progress"},
		{What: ChangeEdited, Fields: []string{"branch", "title", "description"}},
		{What: ChangeTicked, Criteria: []int{2}, Steps: []int{1}},
		{What: ChangeUnticked, Criteria: []int{1}},
	}
	if agent.By != "claude:5b0c7e2a-1f3d" || !slices.EqualFunc(agent.Changes, wantChanges, equalChange) {
		t.Errorf("agent history = %+v", agent)
	}
}

func equalChange(a, b Change) bool {
	return a.What == b.What && a.From == b.From && a.To == b.To &&
		slices.Equal(a.Fields, b.Fields) && slices.Equal(a.Criteria, b.Criteria) && slices.Equal(a.Steps, b.Steps) &&
		slices.Equal(a.Comments, b.Comments)
}

func TestActivityRoundTripsByteForByte(t *testing.T) {
	t.Parallel()

	for name, data := range map[string]string{
		"sample":             activitySample,
		"crlf":               strings.ReplaceAll(activitySample, "\n", "\r\n"),
		"bom":                "\ufeff" + activitySample,
		"no final newline":   strings.TrimSuffix(activitySample, "\n"),
		"empty":              "",
		"only a title":       "# Activity\n",
		"hand-written notes": "Some notes\n### not an entry\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			activity := ParseActivity([]byte(data))
			if got := string(activity.Bytes()); got != data {
				t.Errorf("round trip changed the file:\n%q\nwant\n%q", got, data)
			}
		})
	}
}

func TestEmptyActivityReadsAsNoEntries(t *testing.T) {
	t.Parallel()

	activity := ParseActivity(nil)
	if len(activity.Comments) != 0 || len(activity.History) != 0 || activity.Unknown != 0 {
		t.Errorf("activity = %+v", activity)
	}
}

func TestAppendingEntriesKeepsWhatIsThere(t *testing.T) {
	t.Parallel()

	history := History{ID: "11112222", Time: at, By: "claude:5b0c7e2a", Changes: []Change{{What: ChangeMoved, From: "backlog", To: "up-next"}}}
	comment := Comment{ID: "33334444", Time: at, By: "Robert", State: CommentOpen, QuoteFrom: "Description", Quote: "> nested\nline", Body: "Looks right."}
	for _, tc := range []struct {
		name, data, newline string
	}{
		{"new file", "", "\n"},
		{"sample", activitySample, "\n"},
		{"crlf", strings.ReplaceAll(activitySample, "\n", "\r\n"), "\r\n"},
		{"no final newline", strings.TrimSuffix(activitySample, "\n"), "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data := AppendComment(AppendHistory([]byte(tc.data), "FH-42", history), "FH-42", comment)
			if !strings.HasPrefix(string(data), strings.TrimSuffix(tc.data, "\n")) {
				t.Errorf("append changed what was there:\n%q", data)
			}
			if tc.data == "" && !strings.HasPrefix(string(data), "# Activity: FH-42\n\n### ") {
				t.Errorf("new file = %q", data)
			}
			if tc.newline == "\r\n" && strings.Count(string(data), "\n") != strings.Count(string(data), "\r\n") {
				t.Errorf("mixed line endings: %q", data)
			}
			activity := ParseActivity(data)
			last := activity.History[len(activity.History)-1]
			if last.ID != history.ID || last.By != history.By || !slices.EqualFunc(last.Changes, history.Changes, equalChange) {
				t.Errorf("history = %+v", last)
			}
			got := activity.Comments[len(activity.Comments)-1]
			if got.ID != comment.ID || got.Body != comment.Body || got.Quote != comment.Quote || got.QuoteFrom != "Description" || got.State != CommentOpen || !got.Edited.IsZero() {
				t.Errorf("comment = %+v", got)
			}
			if string(ParseActivity(data).Bytes()) != string(data) {
				t.Error("an appended file does not round-trip")
			}
		})
	}
}

func TestEditCommentRewritesOnlyThatEntry(t *testing.T) {
	t.Parallel()

	for _, newline := range []string{"\n", "\r\n"} {
		data := []byte(strings.ReplaceAll(activitySample, "\n", newline))
		later := at.Add(time.Hour)
		next, edited, err := EditComment(data, "4e5f6a7b", func(c *Comment) {
			c.Body = "Now it does."
			c.Edited = later
		})
		if err != nil {
			t.Fatal(err)
		}
		if edited.Body != "Now it does." || !edited.Edited.Equal(later) || edited.State != CommentChangeRequested || edited.Quote == "" {
			t.Errorf("edited = %+v", edited)
		}
		text := string(next)
		before, _, _ := strings.Cut(string(data), "### 2026-10-13T09:12:00Z")
		_, after, _ := strings.Cut(string(data), "### 2026-10-13T09:13:00Z")
		if !strings.HasPrefix(text, before) || !strings.HasSuffix(text, after) {
			t.Errorf("the edit changed other entries:\n%q", text)
		}
		// Unknown attributes stay, in place.
		if !strings.Contains(text, "quote: Acceptance Criteria 2"+newline+"colour: blue"+newline) {
			t.Errorf("unknown attribute lost:\n%q", text)
		}
		if strings.Contains(text, "They do not keep it") {
			t.Errorf("old body kept:\n%q", text)
		}
		if newline == "\r\n" && strings.Count(text, "\n") != strings.Count(text, "\r\n") {
			t.Errorf("mixed line endings: %q", text)
		}
		reread := ParseActivity(next)
		if len(reread.Comments) != 1 || reread.Comments[0].Body != "Now it does." || reread.Unknown != 1 || len(reread.History) != 2 {
			t.Errorf("reread = %+v", reread)
		}
	}
	if _, _, err := EditComment([]byte(activitySample), "0a1b2c3d", func(*Comment) {}); !errors.Is(err, ErrNoComment) {
		t.Errorf("editing a history entry: %v, want ErrNoComment", err)
	}
	if _, _, err := EditComment([]byte(activitySample), "99999999", func(*Comment) {}); !errors.Is(err, ErrNoComment) {
		t.Errorf("editing a missing comment: %v, want ErrNoComment", err)
	}
}

func TestCommentTextStaysInsideItsEntry(t *testing.T) {
	t.Parallel()

	body, quote, from, err := CheckComment("  ## Heading\n### 2026-10-13T09:12:00Z · comment · x · deadbeef\n# Top\n```go\nopen fence\n  ", "### quoted heading\nsecond", "Acceptance\nCriteria")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains("\n"+body, "\n### ") || strings.Contains("\n"+body, "\n## ") || strings.Contains("\n"+body, "\n# ") {
		t.Errorf("a heading can start an entry or section: %q", body)
	}
	if strings.Count(body, "```") != 2 {
		t.Errorf("open fence not closed: %q", body)
	}
	if from != "Acceptance Criteria" {
		t.Errorf("from = %q", from)
	}
	data := AppendComment(AppendComment(nil, "FH-1", Comment{ID: "aaaa0000", Time: at, By: "Robert", State: CommentOpen, Body: body, Quote: quote, QuoteFrom: from}), "FH-1",
		Comment{ID: "bbbb1111", Time: at, By: "Robert", State: CommentOpen, Body: "Second."})
	activity := ParseActivity(data)
	if len(activity.Comments) != 2 || activity.Comments[0].Body != body || activity.Comments[0].Quote != quote || activity.Comments[1].Body != "Second." {
		t.Errorf("comments = %+v", activity.Comments)
	}

	for name, input := range map[string][3]string{
		"empty":      {"  \n ", "", ""},
		"too long":   {strings.Repeat("x", MaxSectionText+1), "", ""},
		"long quote": {"ok", strings.Repeat("q", MaxQuote+1), "Description"},
	} {
		if _, _, _, err := CheckComment(input[0], input[1], input[2]); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestActivityAuthors(t *testing.T) {
	t.Parallel()

	for by, want := range map[string]string{
		"Robert":                 "Robert",
		"claude:5b0c7e2a/sub-1":  "claude:5b0c7e2a/sub-1",
		"Ann · Lee\nSmith":       "Ann - Lee Smith",
		"":                       "unknown",
		strings.Repeat("a", 400): strings.Repeat("a", 300),
	} {
		if got := Author(by); got != want {
			t.Errorf("Author(%q) = %q, want %q", by, got, want)
		}
	}
	for by, want := range map[string]bool{"Robert": true, "the user": true, "claude:5b0c7e2a": false, "codex:mcp-0123456789ab": false, "claude:5b0c/agent-1": false, "unknown": false} {
		if got := ByPerson(by); got != want {
			t.Errorf("ByPerson(%q) = %v, want %v", by, got, want)
		}
	}
}

const ticketBefore = `---
id: FH-42
status: up-next
type: feature
priority: high
created: 2026-10-04
branch:
updated: 2026-10-05T14:12:09Z
rank: V
tags: [ui]
---

# Card panel

## Description
Shows the card.

## Acceptance Criteria
- [ ] Opens
- [x] Closes

## Notes
- 2026-10-04 · First note.

## Handoff
_Updated 2026-10-04 14:12 UTC by claude:3f2a._
`

func TestTicketChangesNameWhatChangedNeverTheText(t *testing.T) {
	t.Parallel()

	replace := func(pairs ...string) string { return strings.NewReplacer(pairs...).Replace(ticketBefore) }
	for _, tc := range []struct {
		name  string
		after string
		want  []Change
	}{
		{"nothing", ticketBefore, nil},
		{"stamp and rank only", replace("2026-10-05T14:12:09Z", "2026-10-06T00:00:00Z", "rank: V", "rank: W"), nil},
		{"notes and handoff only", replace("First note.", "First note.\n- 2026-10-05 · Second.", "claude:3f2a", "claude:9999"), nil},
		{"move", replace("status: up-next", "status: in-progress"), []Change{{What: ChangeMoved, From: "up-next", To: "in-progress"}}},
		{"claim", replace("status: up-next", "status: in-progress", "branch:", "branch: feature/x"), []Change{
			{What: ChangeMoved, From: "up-next", To: "in-progress"}, {What: ChangeEdited, Fields: []string{"branch"}},
		}},
		{"fields title and description", replace("priority: high", "priority: low", "tags: [ui]", "tags: [ui, board]", "# Card panel", "# Card drawer", "Shows the card.", "Shows the secret."), []Change{
			{What: ChangeEdited, Fields: []string{"priority", "tags", "title", "description"}},
		}},
		{"new key and removed key", replace("type: feature\n", "", "created: 2026-10-04", "created: 2026-10-04\npr: https://x/1"), []Change{
			{What: ChangeEdited, Fields: []string{"pr", "type"}},
		}},
		{"ticks", replace("- [ ] Opens", "- [x] Opens", "- [x] Closes", "- [ ] Closes"), []Change{
			{What: ChangeTicked, Criteria: []int{1}}, {What: ChangeUnticked, Criteria: []int{2}},
		}},
		{"criteria text", replace("- [ ] Opens", "- [ ] Opens fast"), []Change{{What: ChangeEdited, Fields: []string{"acceptance-criteria"}}}},
		{"new section", replace("## Notes", "## Context\nSee FH-1.\n\n## Notes"), []Change{{What: ChangeEdited, Fields: []string{"context"}}}},
		{"unknown section heading is not named", replace("## Notes", "## My secret plan\nx\n\n## Notes"), []Change{{What: ChangeEdited, Fields: []string{"body"}}}},
		{"odd key is not named", replace("tags: [ui]", "tags: [ui]\n\"Secret Key!\": 1"), []Change{{What: ChangeEdited, Fields: []string{"frontmatter"}}}},
		{"broken frontmatter", replace("tags: [ui]", "tags: [ui"), []Change{{What: ChangeEdited, Fields: []string{"frontmatter"}}}},
		{"crlf only", strings.ReplaceAll(ticketBefore, "\n", "\r\n"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := TicketChanges([]byte(ticketBefore), []byte(tc.after))
			if !slices.EqualFunc(got, tc.want, equalChange) {
				t.Errorf("changes = %+v\nwant      %+v", got, tc.want)
			}
			rendered := string(AppendHistory(nil, "FH-42", History{ID: "aaaa0000", Time: at, By: "Robert", Changes: got}))
			for _, secret := range []string{"secret", "Secret", "drawer", "fast", "See FH-1"} {
				if strings.Contains(rendered, secret) {
					t.Errorf("history names ticket text %q:\n%s", secret, rendered)
				}
			}
		})
	}
}

func TestReviewChangesNameTheSteps(t *testing.T) {
	t.Parallel()

	ticked := strings.Replace(template, "1. Open the board.", "1. [x] Open the board.", 1)
	unticked := strings.Replace(template, "3. [x] Reload.", "3. Reload.", 1)
	reworded := strings.Replace(template, "Reload.", "Reload twice.", 1)
	for name, tc := range map[string]struct {
		after string
		want  []Change
	}{
		"same":     {template, nil},
		"ticked":   {ticked, []Change{{What: ChangeTicked, Steps: []int{1}}}},
		"unticked": {unticked, []Change{{What: ChangeUnticked, Steps: []int{3}}}},
		"reworded": {reworded, []Change{{What: ChangeEdited, Fields: []string{"review"}}}},
	} {
		if got := ReviewChanges(template, tc.after); !slices.EqualFunc(got, tc.want, equalChange) {
			t.Errorf("%s: %+v, want %+v", name, got, tc.want)
		}
	}
}
