package mdfile

import (
	"strings"
	"testing"
)

// TaskItems splits a section at its top-level task items (FH-69): each
// item takes the lines up to the next one, the last only its indented
// continuation, and the parts joined are the content again.
func TestTaskItems(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, content string
		head          string
		items         []string
		tail          string
	}{
		{name: "plain list", content: "- [ ] One\n- [x] Two\n", items: []string{"- [ ] One\n", "- [x] Two\n"}},
		{name: "intro and outro", content: "\nThese hold.\n\n- [ ] One\n- [X] Two\n\nAnd that is all.\n",
			head: "\nThese hold.\n\n", items: []string{"- [ ] One\n", "- [X] Two\n"}, tail: "\nAnd that is all.\n"},
		{name: "continuation lines", content: "- [ ] One\n  more of one\n\n  still one\n- [ ] Two\n  more of two\n\nAfter.\n",
			items: []string{"- [ ] One\n  more of one\n\n  still one\n", "- [ ] Two\n  more of two\n"}, tail: "\nAfter.\n"},
		{name: "loose list", content: "- [ ] One\n\n- [ ] Two\n", items: []string{"- [ ] One\n\n", "- [ ] Two\n"}},
		{name: "trailing blank lines", content: "- [ ] One\n\n\n", items: []string{"- [ ] One\n"}, tail: "\n\n"},
		{name: "no final newline", content: "- [ ] One\n- [ ] Two", items: []string{"- [ ] One\n", "- [ ] Two"}},
		{name: "CRLF", content: "- [ ] One\r\n- [x] Two\r\n", items: []string{"- [ ] One\r\n", "- [x] Two\r\n"}},
		{name: "fenced boxes are not items", content: "```\n- [ ] not one\n```\n- [ ] One\n",
			head: "```\n- [ ] not one\n```\n", items: []string{"- [ ] One\n"}},
		{name: "nested boxes belong to their item", content: "- [ ] One\n  - [ ] part\n- [ ] Two\n",
			items: []string{"- [ ] One\n  - [ ] part\n", "- [ ] Two\n"}},
		{name: "none", content: "None.\n", head: "None.\n"},
		{name: "empty", content: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			head, items, tail := TaskItems(tc.content)
			if head != tc.head || strings.Join(items, "|") != strings.Join(tc.items, "|") || tail != tc.tail {
				t.Errorf("TaskItems(%q) =\n%q %q %q\nwant\n%q %q %q", tc.content, head, items, tail, tc.head, tc.items, tc.tail)
			}
			if joined := head + strings.Join(items, "") + tail; joined != tc.content {
				t.Errorf("parts join to %q, want %q", joined, tc.content)
			}
		})
	}
}
