package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// pruneExpired expires every project's old files (FH-82), reporting
// failures to diagnostics: event files older than days
// (event_retention_days, STO-5) with the session inboxes last written on
// those days (agent-protocol §4), and the activity state of sessions no
// hook has written for stale_hours, whose runs the stale rule has ended
// (agent-protocol §3). Sessions that never report their end, a crash or
// any Codex session, leave nothing behind for longer than that. It then
// moves inboxes an older binary left in the old answers/ location into
// the inbox the prompt hook checks (D54).
func pruneExpired(files *store.Store, now time.Time, days int, diagnostics io.Writer) {
	projects, err := files.Projects()
	if err != nil {
		return
	}
	log := events.New(files)
	stale := now.Add(-runs.DefaultSettings().Stale)
	for _, project := range projects {
		if _, err := log.Prune(project, now, days); err != nil {
			fmt.Fprintf(diagnostics, "flashheart: expire old events in %s: %v\n", project, err)
		}
		if _, err := files.PruneActivity(project, stale); err != nil {
			fmt.Fprintf(diagnostics, "flashheart: expire stale activity state in %s: %v\n", project, err)
		}
		if _, err := files.DrainLegacyInboxes(project); err != nil {
			fmt.Fprintf(diagnostics, "flashheart: move old inboxes in %s: %v\n", project, err)
		}
	}
}

// keepPruned prunes at startup and then daily until ctx ends.
func keepPruned(ctx context.Context, files *store.Store, days int, diagnostics io.Writer) {
	pruneExpired(files, time.Now(), days, diagnostics)
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pruneExpired(files, time.Now(), days, diagnostics)
		}
	}
}
