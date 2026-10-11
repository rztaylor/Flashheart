package api

import (
	"fmt"
	"net/http"
)

// MetricsResponse is GET /api/metrics?project=: the Overview's headline
// metrics (VIEW-3, FH-52) for a project, or every project without one.
// Since is the human's latest board activity there when LastChange is set,
// otherwise a day ago. Done, Review and Created count distinct tickets;
// CriteriaTicked counts ticket updates that ticked criteria.
type MetricsResponse struct {
	Revision       uint64 `json:"revision"`
	Since          string `json:"since"`
	LastChange     bool   `json:"lastChange"`
	Done           int    `json:"done"`
	Review         int    `json:"review"`
	Created        int    `json:"created"`
	CriteriaTicked int    `json:"criteriaTicked"`
}

func (b boardAPI) metrics(w http.ResponseWriter, r *http.Request) {
	snapshot := b.snapshot(w)
	if snapshot == nil {
		return
	}
	project := r.URL.Query().Get("project")
	if _, ok := snapshot.Project(project); project != "" && !ok {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("No project %s", project))
		return
	}
	metrics := snapshot.Metrics(project)
	writeJSON(w, http.StatusOK, MetricsResponse{
		Revision: snapshot.Revision, Since: timestamp(metrics.Since), LastChange: metrics.LastChange,
		Done: metrics.Done, Review: metrics.Review, Created: metrics.Created, CriteriaTicked: metrics.Criteria,
	})
}
