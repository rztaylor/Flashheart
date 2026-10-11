package api

import (
	"net/http"
	"testing"
	"time"
)

// The Overview's headline metrics count since the human's latest board
// activity in the scope, or over the last day with none (VIEW-3, FH-52).
func TestMetricsEndpointCountsSinceTheHumansLastChange(t *testing.T) {
	t.Parallel()

	agent := `"run":"claude:s9","agent":"claude","project":"alpha"`
	handler := runsAPI(t, time.Date(2026, 10, 4, 13, 30, 0, 0, time.UTC),
		`{"v":1,"ts":"2026-10-04T12:00:00Z",`+agent+`,"kind":"ticket.moved","data":{"ticket":"AL-2","from":"in-progress","to":"review","by":"claude:s9"}}`,
		`{"v":1,"ts":"2026-10-04T12:30:00Z","run":"human","agent":"human","project":"alpha","kind":"ticket.moved","data":{"ticket":"AL-2","from":"review","to":"done","by":"human"}}`,
		`{"v":1,"ts":"2026-10-04T13:10:00Z",`+agent+`,"kind":"ticket.created","data":{"ticket":"AL-9","by":"claude:s9"}}`,
		`{"v":1,"ts":"2026-10-04T13:11:00Z",`+agent+`,"kind":"ticket.updated","data":{"ticket":"AL-3","fields":["criteria","notes"]}}`,
		`{"v":1,"ts":"2026-10-04T13:12:00Z",`+agent+`,"kind":"ticket.moved","data":{"ticket":"AL-3","from":"in-progress","to":"review","by":"claude:s9"}}`,
	)
	want := MetricsResponse{Since: "2026-10-04T12:30:00Z", LastChange: true, Review: 1, Created: 1, CriteriaTicked: 1}
	for _, path := range []string{"/api/metrics", "/api/metrics?project=alpha"} {
		var got MetricsResponse
		getJSON(t, handler, path, http.StatusOK, &got)
		got.Revision = 0
		if got != want {
			t.Errorf("%s = %+v, want %+v", path, got, want)
		}
	}

	var beta MetricsResponse
	getJSON(t, handler, "/api/metrics?project=beta", http.StatusOK, &beta)
	if beta.Revision == 0 || beta.Since != "2026-10-03T13:30:00Z" || beta.LastChange || beta.Review+beta.Done+beta.Created+beta.CriteriaTicked != 0 {
		t.Errorf("beta = %+v, want the last day and nothing", beta)
	}
	getJSON(t, handler, "/api/metrics?project=gamma", http.StatusNotFound, nil)
}
