package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// largeArchive copies the sample root and gives alpha n archived tickets of
// a few KiB each, every tenth depending on the one before it.
func largeArchive(tb testing.TB, n int) string {
	tb.Helper()
	root := filepath.Join(tb.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "boards", "sample"))); err != nil {
		tb.Fatal(err)
	}
	body := strings.Repeat("Some notes on what was done and why, kept for the record.\n", 30)
	for index := range n {
		id := fmt.Sprintf("AL-%d", 1000+index)
		depends := "[]"
		if index%10 == 9 {
			depends = fmt.Sprintf("[AL-%d]", 1000+index-1)
		}
		folder := filepath.Join(root, "alpha", ".archive", "tickets", id+"-archived")
		if err := os.MkdirAll(folder, 0o755); err != nil {
			tb.Fatal(err)
		}
		content := fmt.Sprintf("---\nid: %s\nstatus: done\ntype: feature\npriority: low\ncreated: 2026-09-01\nupdated: 2026-09-02T10:00:00Z\ndepends-on: %s\ndepends-on-workstreams: []\ntags: [archive, bench]\n---\n\n# Archived ticket %d\n\n## Description\n\n%s\n## Acceptance Criteria\n\n- [x] Done\n- [x] Also done\n\n## Notes\n\n%s", id, depends, index, body, body)
		if err := os.WriteFile(filepath.Join(folder, id+"-archived.md"), []byte(content), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	return root
}

// BenchmarkDeleteArchivedLargeArchive times a permanent delete's plan and
// the whole delete on a root whose project has 2,000 archived tickets
// (FH-26). Each iteration archives a fresh ticket that BE-1 depends on.
// Run with: go test ./internal/store -run '^$' -bench DeleteArchivedLargeArchive -benchtime 20x
func BenchmarkDeleteArchivedLargeArchive(b *testing.B) {
	root := largeArchive(b, 2000)
	s, err := Open(root)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { s.Close() })
	next := 5000
	target := func() string {
		next++
		id := fmt.Sprintf("AL-%d", next)
		folder := filepath.Join(root, "alpha", ".archive", "tickets", id+"-target")
		if err := os.MkdirAll(folder, 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, id+"-target.md"), []byte("---\nid: "+id+"\nstatus: done\n---\n\n# Target\n"), 0o644); err != nil {
			b.Fatal(err)
		}
		be1 := filepath.Join(root, "beta", "tickets", "BE-1-hello", "BE-1-hello.md")
		data, err := os.ReadFile(be1)
		if err != nil {
			b.Fatal(err)
		}
		lines := strings.Split(string(data), "\n")
		for index, line := range lines {
			if strings.HasPrefix(line, "depends-on:") {
				lines[index] = "depends-on: [" + id + "]"
			}
		}
		if err := os.WriteFile(be1, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			b.Fatal(err)
		}
		return id
	}
	// run times step on a fresh target each iteration; step returns the
	// time of the part it measures.
	run := func(b *testing.B, metric string, step func(id string) (time.Duration, error)) {
		var total time.Duration
		for b.Loop() {
			took, err := step(target())
			if err != nil {
				b.Fatal(err)
			}
			total += took
		}
		b.ReportMetric(float64(total.Microseconds())/1000/float64(b.N), metric)
	}
	timed := func(f func() error) (time.Duration, error) {
		started := time.Now()
		err := f()
		return time.Since(started), err
	}
	b.Run("plan", func(b *testing.B) {
		run(b, "ms/plan", func(id string) (time.Duration, error) {
			return timed(func() error { _, err := s.PlanDelete("alpha", id); return err })
		})
	})
	// The plan is the preview the user saw; only the delete is timed.
	b.Run("delete", func(b *testing.B) {
		run(b, "ms/delete", func(id string) (time.Duration, error) {
			plan, err := s.PlanDelete("alpha", id)
			if err != nil {
				return 0, err
			}
			return timed(func() error { _, err := s.DeleteArchived("alpha", id, plan.Token); return err })
		})
	})
	// The part of the delete that holds the root lock, which ticket
	// creation, key choice and hooks for new repositories wait on.
	b.Run("locked", func(b *testing.B) {
		run(b, "ms/locked", func(id string) (time.Duration, error) {
			plan, err := s.PlanDelete("alpha", id)
			if err != nil {
				return 0, err
			}
			return timed(func() error { _, err := s.deleteLocked(plan); return err })
		})
	})
}
