package board

import (
	"slices"
	"strings"
	"testing"
)

const template = `# Review: Board

## Summary
Done.

## How to Verify
### Prerequisites
A board with tickets.
### Steps
1. Open the board.
2. Drag a card ` + "`down`" + `.
   It lands where dropped.
3. [x] Reload.
### Expected Results
The order holds.

## Risks / Things to Watch
- None.
`

func stepTexts(steps []ReviewStep) []string {
	var out []string
	for _, step := range steps {
		out = append(out, step.Text)
	}
	return out
}

func TestParseReviewSplitsHowToVerify(t *testing.T) {
	t.Parallel()

	review := ParseReview(template)
	if got := stepTexts(review.Steps); !slices.Equal(got, []string{"Open the board.", "Drag a card `down`.\nIt lands where dropped.", "Reload."}) {
		t.Fatalf("steps = %q", got)
	}
	if review.Steps[0].Done || review.Steps[1].Done || !review.Steps[2].Done {
		t.Errorf("done = %+v", review.Steps)
	}
	if !strings.HasSuffix(review.Before, "## Summary\nDone.\n\n") || strings.Contains(review.Before, "How to Verify") {
		t.Errorf("before = %q", review.Before)
	}
	if review.Intro != "### Prerequisites\nA board with tickets.\n### Steps\n" {
		t.Errorf("intro = %q", review.Intro)
	}
	if review.Outro != "### Expected Results\nThe order holds.\n\n" {
		t.Errorf("outro = %q", review.Outro)
	}
	if review.After != "## Risks / Things to Watch\n- None.\n" {
		t.Errorf("after = %q", review.After)
	}
	if review.Before+"## How to Verify\n"+review.Intro+"1. Open the board.\n2. Drag a card `down`.\n   It lands where dropped.\n3. [x] Reload.\n"+review.Outro+review.After != template {
		t.Error("the parts do not rebuild the review")
	}
}

func TestParseReviewWithoutStepsHeadingOrSection(t *testing.T) {
	t.Parallel()

	plain := "## How to Verify\n\n1. Run `scripts/check.sh`.\n2. Symlink a ticket.\n\n![Store tests](files/a.png)\n\n## Risks\n- x\n"
	review := ParseReview(plain)
	if got := stepTexts(review.Steps); !slices.Equal(got, []string{"Run `scripts/check.sh`.", "Symlink a ticket."}) {
		t.Fatalf("steps = %q", got)
	}
	if review.Outro != "\n![Store tests](files/a.png)\n\n" {
		t.Errorf("outro = %q", review.Outro)
	}
	for _, markdown := range []string{"## Summary\nNo section.\n", "## How to Verify\nJust look.\n", "```\n## How to Verify\n1. fenced\n```\n"} {
		if steps := ParseReview(markdown).Steps; len(steps) != 0 {
			t.Errorf("ParseReview(%q) steps = %q", markdown, stepTexts(steps))
		}
	}
}

func TestSetReviewStep(t *testing.T) {
	t.Parallel()

	ticked, err := SetReviewStep(template, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ticked, "1. [x] Open the board.\n") || !ParseReview(ticked).Steps[0].Done {
		t.Errorf("ticked:\n%s", ticked)
	}
	cleared, _ := SetReviewStep(ticked, 2, false)
	if !strings.Contains(cleared, "3. [ ] Reload.\n") {
		t.Errorf("cleared:\n%s", cleared)
	}
	bullets := "## How to Verify\n- [ ] One\n- Two\n"
	got, _ := SetReviewStep(bullets, 1, true)
	if got != "## How to Verify\n- [ ] One\n- [x] Two\n" {
		t.Errorf("bullets = %q", got)
	}
	if _, err := SetReviewStep(template, 3, true); err == nil {
		t.Error("a step that does not exist should fail")
	}
	crlf := strings.ReplaceAll(template, "\n", "\r\n")
	if got, _ := SetReviewStep(crlf, 0, true); !strings.Contains(got, "1. [x] Open the board.\r\n") {
		t.Error("CRLF line endings were not kept")
	}
}
