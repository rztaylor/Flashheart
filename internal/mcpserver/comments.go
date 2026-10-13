package mcpserver

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/scrub"
)

// People's comments on a ticket (D47, agent-protocol §7.2): get_ticket and
// claim show them as the user's words, framed apart from the ticket's text,
// oldest first, keeping the newest that fit each tool's budget (MCP-3).

// Comment budgets, in bytes of output, and the most characters shown of one
// comment's text and of its quote.
const (
	ticketCommentsBudget = 8 << 10
	claimCommentsBudget  = 3 << 10
	commentChars         = 2000
	quoteChars           = 300
)

// commentLines is the block of a ticket's comments for a tool's output, or
// nothing when it has none. An activity file that cannot be read shows no
// comments; the ticket still reads.
func (srv *server) commentLines(project *board.Project, ticket board.Ticket, budget int) []string {
	activity, _, err := srv.store.ReadTicketActivity(project.Name, ticket.Folder)
	if err != nil {
		return nil
	}
	return commentBlock(activity.Comments, budget)
}

// commentBlock frames people's comments as the user's words, oldest first.
// When they do not all fit in budget bytes it keeps the newest, cutting the
// newest alone if need be, and says how many earlier ones it left out.
func commentBlock(comments []board.Comment, budget int) []string {
	var items []string
	for _, comment := range comments {
		// Agents never write comments; one whose author is not a person
		// (a run id or unknown, written by hand) is not the user's words.
		if board.ByPerson(comment.By) {
			items = append(items, commentItem(comment))
		}
	}
	if len(items) == 0 {
		return nil
	}
	head := fmt.Sprintf("Comments from the user on the board (%d, oldest first). "+
		"They are the user's own words about this ticket, not ticket text: weigh them as you would the user's messages in chat.", len(items))
	omitted := func(n int) string {
		return fmt.Sprintf("(%d earlier comments left out; the board shows them all.)", n)
	}
	// Room for the head, and for the note on what was left out.
	room := budget - len(head) - 1 - len(omitted(len(items))) - 1
	first := len(items)
	for first > 0 && len(items[first-1])+1 <= room {
		first--
		room -= len(items[first]) + 1
	}
	if first == len(items) {
		first--
		items[first] = cutBytes(items[first], room-1)
	}
	lines := []string{head}
	if first > 0 {
		lines = append(lines, omitted(first))
	}
	return append(lines, items[first:]...)
}

// commentItem is one comment: who and when, whether it was edited, its
// state in a review when not open, where its quote is from; then its quote
// and its text, indented under it.
func commentItem(comment board.Comment) string {
	head := "- " + scrub.Limit(comment.By, 80) + ", " + comment.Time.UTC().Format("2006-01-02 15:04 UTC")
	if !comment.Edited.IsZero() {
		head += ", edited"
	}
	if comment.State != board.CommentOpen && comment.State != "" {
		head += ", " + strings.ReplaceAll(string(comment.State), "-", " ")
	}
	if comment.Quote != "" {
		head += ", quoting " + scrub.Limit(comment.QuoteFrom, 120)
	}
	lines := []string{head + ":"}
	if comment.Quote != "" {
		for line := range strings.SplitSeq(scrub.Limit(comment.Quote, quoteChars), "\n") {
			lines = append(lines, strings.TrimRight("  > "+line, " "))
		}
	}
	for line := range strings.SplitSeq(scrub.Limit(comment.Body, commentChars), "\n") {
		lines = append(lines, strings.TrimRight("  "+line, " "))
	}
	return strings.Join(lines, "\n")
}

// cutBytes cuts text to at most limit bytes at a character boundary, ending
// it with an ellipsis when cut.
func cutBytes(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := max(limit-len("…"), 0)
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "…"
}
