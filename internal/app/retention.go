package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// pruneEvents deletes every project's event files older than days
// (event_retention_days, STO-5), reporting failures to diagnostics.
func pruneEvents(files *store.Store, now time.Time, days int, diagnostics io.Writer) {
	projects, err := files.Projects()
	if err != nil {
		return
	}
	log := events.New(files)
	for _, project := range projects {
		if _, err := log.Prune(project, now, days); err != nil {
			fmt.Fprintf(diagnostics, "flashheart: expire old events in %s: %v\n", project, err)
		}
	}
}

// keepEventsPruned prunes at startup and then daily until ctx ends.
func keepEventsPruned(ctx context.Context, files *store.Store, days int, diagnostics io.Writer) {
	pruneEvents(files, time.Now(), days, diagnostics)
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pruneEvents(files, time.Now(), days, diagnostics)
		}
	}
}
