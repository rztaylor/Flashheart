package index

import (
	"time"

	"github.com/rztaylor/flashheart/internal/events"
)

// MetricsFallback is how far back the headline metrics count when the
// scope has no recorded human activity (VIEW-3).
const MetricsFallback = 24 * time.Hour

// Metrics are the Overview's headline metrics (VIEW-3, FH-52): what agents
// changed since Since. LastChange means Since is the human's latest board
// activity in the scope; otherwise none is recorded and Since is a day
// before the snapshot was built.
type Metrics struct {
	Since      time.Time
	LastChange bool
	events.ChangeCounts
}

// Metrics counts a project's changes, or every project's when project is
// "", since the human's latest board activity there. The human's own
// changes are that activity, so only what came after it counts.
func (s *Snapshot) Metrics(project string) Metrics {
	metrics := Metrics{Since: s.BuiltAt.Add(-MetricsFallback)}
	if activity, ok := s.LatestHuman(project); ok {
		metrics.Since, metrics.LastChange = activity.Time, true
	}
	metrics.ChangeCounts = events.CountChanges(s.changes, project, metrics.Since)
	return metrics
}
