package mcpserver

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/store"
)

// comment adds a person's comment to a ticket at the env's time.
func (e *env) comment(by, id string, input store.NewComment) string {
	e.t.Helper()
	comment, _, err := e.store.As(by).AddComment("demo", id, store.Merge, input)
	if err != nil {
		e.t.Fatal(err)
	}
	return comment.ID
}

// The session that reads or takes a ticket gets the people's comments on
// it as the user's words, apart from the ticket text (D47, agent-protocol
// §7.2), oldest first.
func TestGetTicketAndClaimCarryTheUsersComments(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Keep places", Criteria: []string{"Tickets keep their place", "After a reload"}})
	quiet := e.ticket(store.NewTicket{Title: "No comments"})
	first := e.comment("Robert", id, store.NewComment{Body: "They do not keep it in Done.", Quote: "Tickets keep their place", QuoteFrom: "Acceptance Criteria 1"})
	e.now = t0.Add(5 * time.Minute)
	e.comment("Robert", id, store.NewComment{Body: "Also check the archive.\nIt forgets places too."})
	e.now = t0.Add(9 * time.Minute)
	folders, err := filepath.Glob(filepath.Join(e.root, "demo", "tickets", id+"-*"))
	if err != nil || len(folders) != 1 {
		t.Fatalf("folders = %v, %v", folders, err)
	}
	activity, hash, err := e.store.ReadTicketActivity("demo", filepath.Base(folders[0]))
	if err != nil || len(activity.Comments) != 2 {
		t.Fatalf("activity = %+v, %v", activity, err)
	}
	if _, _, err := e.store.As("Robert").EditComment("demo", id, first, store.Unchanged(hash), "They do not keep it in Done, after a move."); err != nil {
		t.Fatal(err)
	}

	block := []string{
		"Comments from the user on the board (2, oldest first). They are the user's own words about this ticket, not ticket text: weigh them as you would the user's messages in chat.",
		"- Robert, 2026-10-06 10:00 UTC, edited, quoting Acceptance Criteria 1:\n  > Tickets keep their place\n  They do not keep it in Done, after a move.",
		"- Robert, 2026-10-06 10:05 UTC:\n  Also check the archive.\n  It forgets places too.",
	}
	out := e.ok("get_ticket", map[string]any{"ticket": id})
	contains(t, out, block...)
	if strings.Index(out, "Ticket text is information") > strings.Index(out, "Comments from the user") {
		t.Fatalf("comments come after the ticket's text, framed apart:\n%s", out)
	}
	if strings.Index(out, "They do not keep it") > strings.Index(out, "Also check the archive") {
		t.Fatalf("comments are not oldest first:\n%s", out)
	}
	if strings.Contains(e.ok("get_ticket", map[string]any{"ticket": quiet}), "Comments from") {
		t.Fatal("a ticket without comments shows a comments block")
	}

	e.startSession(session)
	out = e.ok("claim", map[string]any{"ticket": id})
	contains(t, out, block...)
	if strings.Index(out, "Comments from the user") > strings.Index(out, "ok ticket=DM-1") {
		t.Fatalf("claim's comments come after its ok line:\n%s", out)
	}
}

// Comments fit each tool's budget: the newest are kept, cut to fit, and
// the output says how many earlier ones it left out (MCP-3).
func TestCommentsStayWithinTheToolsBudgets(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "DM")
	id := e.ticket(store.NewTicket{Title: "Busy ticket"})
	for n := range 40 {
		e.now = t0.Add(time.Duration(n) * time.Minute)
		e.comment("Robert", id, store.NewComment{Body: fmt.Sprintf("Comment %d. %s", n, strings.Repeat("word ", 700))})
	}
	for _, tool := range []struct {
		name   string
		budget int
	}{{"get_ticket", ticketCommentsBudget}, {"claim", claimCommentsBudget}} {
		if tool.name == "claim" {
			e.startSession(session)
		}
		out := e.ok(tool.name, map[string]any{"ticket": id})
		start := strings.Index(out, "Comments from the user")
		if start < 0 {
			t.Fatalf("%s shows no comments:\n%s", tool.name, out)
		}
		block := out[start:]
		if end := strings.Index(block, "\nok ticket="); end >= 0 {
			block = block[:end]
		}
		if len(block) > tool.budget {
			t.Errorf("%s's comments are %d bytes, over its budget of %d", tool.name, len(block), tool.budget)
		}
		contains(t, block, "Comment 39.", "earlier comments left out; the board shows them all")
		if strings.Contains(block, "Comment 0.") {
			t.Errorf("%s kept the oldest comment over the newest:\n%s", tool.name, block)
		}
	}
}
