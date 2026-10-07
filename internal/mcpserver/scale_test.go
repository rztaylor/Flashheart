package mcpserver

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// scaleRoot writes an NFR-1 root: projects of tickets each, with demo (the
// caller's project) among them.
func scaleRoot(tb testing.TB, e *env, projects, tickets int) {
	tb.Helper()
	for p := range projects {
		name, key := fmt.Sprintf("p%02d", p), fmt.Sprintf("P%02d", p)
		if p == 0 {
			name, key = "demo", "DM"
		}
		if p > 0 {
			write(tb, filepath.Join(e.root, name, "project.yaml"), "key: "+key+"\n")
		}
		for n := 1; n <= tickets; n++ {
			id := fmt.Sprintf("%s-%d", key, n)
			folder := id + "-ticket"
			content := fmt.Sprintf("---\nid: %s\nstatus: backlog\ntype: feature\npriority: medium\ncreated: 2026-10-01\n---\n# Ticket %d\n\n## Description\n\nGenerated.\n\n## Acceptance Criteria\n\n- [ ] One\n", id, n)
			path := filepath.Join(e.root, name, "tickets", folder, folder+".md")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				tb.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				tb.Fatal(err)
			}
		}
	}
}

// BenchmarkToolsAtScale times read and write tools on an NFR-1 root (50
// projects of 1,000 tickets). Run with: go test ./internal/mcpserver -run
// '^$' -bench ToolsAtScale -benchtime 10x
func BenchmarkToolsAtScale(b *testing.B) {
	e := newEnv(b, "DM")
	scaleRoot(b, e, 50, 1000)
	e.startSession(session)
	for _, call := range []struct {
		tool string
		args map[string]any
	}{
		{"board_context", nil},
		{"get_ticket", map[string]any{"ticket": "P07-500"}},
		{"list_tickets", map[string]any{"limit": 5}},
		{"update_ticket", map[string]any{"ticket": "DM-10", "append_notes": "x"}},
	} {
		b.Run(call.tool, func(b *testing.B) {
			for b.Loop() {
				started := time.Now()
				if _, failed := e.call(call.tool, call.args); failed {
					b.Fatalf("%s failed", call.tool)
				}
				b.ReportMetric(float64(time.Since(started).Milliseconds()), "ms/call")
			}
		})
	}
}
