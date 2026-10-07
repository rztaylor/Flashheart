package api

import (
	"net/http"
	"slices"
	"testing"
)

func archivedIDs(t *testing.T, handler http.Handler, project string) []ArchivedJSON {
	t.Helper()
	var response ArchiveResponse
	getJSON(t, handler, "/api/projects/"+project+"/archive", http.StatusOK, &response)
	return response.Tickets
}

func TestArchiveListsArchivedTickets(t *testing.T) {
	t.Parallel()

	handler, _ := writableAPI(t)
	send(t, handler, http.MethodPost, "/api/tickets/AL-3/archive", struct{}{}, http.StatusOK, nil)
	tickets := archivedIDs(t, handler, "alpha")
	if len(tickets) != 2 || tickets[0].ID != "AL-3" || tickets[1].ID != "AL-8" {
		t.Fatalf("archived = %+v", tickets)
	}
	if tickets[0].Column != "in-progress" || tickets[0].Title == "" || tickets[0].Archived == "" {
		t.Errorf("AL-3 = %+v", tickets[0])
	}
	var projects ProjectsResponse
	getJSON(t, handler, "/api/projects", http.StatusOK, &projects)
	for _, project := range projects.Projects {
		if project.Name == "alpha" && project.Archived != 2 {
			t.Errorf("alpha archived count = %d", project.Archived)
		}
	}
	getJSON(t, handler, "/api/projects/nowhere/archive", http.StatusNotFound, nil)
}

func TestDeleteArchivedTicket(t *testing.T) {
	t.Parallel()

	handler, _ := writableAPI(t)
	send(t, handler, http.MethodPost, "/api/tickets/AL-3/archive", struct{}{}, http.StatusOK, nil)
	var plan DeletePlanJSON
	getJSON(t, handler, "/api/projects/alpha/archive/AL-3", http.StatusOK, &plan)
	if !slices.Contains(plan.Files, "AL-3-card-panel.md") || plan.Token == "" {
		t.Errorf("plan = %+v", plan)
	}
	if len(plan.Tickets) != 2 || plan.Tickets[0].ID != "AL-4" || plan.Tickets[1].ID != "BE-1" || plan.Tickets[1].Project != "beta" {
		t.Errorf("plan tickets = %+v", plan.Tickets)
	}
	if len(plan.Workstreams) != 1 || plan.Workstreams[0].Slug != "board-ui" || plan.Workstreams[0].Title == "" {
		t.Errorf("plan workstreams = %+v", plan.Workstreams)
	}

	deletePath := "/api/projects/alpha/archive/AL-3/delete"
	// The typed id must match, and the token must be current.
	send(t, handler, http.MethodPost, deletePath, DeleteRequest{Token: plan.Token, Confirm: "AL-30"}, http.StatusBadRequest, nil)
	var conflict struct{ Error struct{ Code string } }
	send(t, handler, http.MethodPost, deletePath, DeleteRequest{Token: "stale", Confirm: "AL-3"}, http.StatusConflict, &conflict)
	if conflict.Error.Code != "conflict" {
		t.Errorf("stale token = %+v", conflict)
	}
	send(t, handler, http.MethodPost, deletePath, DeleteRequest{Token: plan.Token, Confirm: "AL-3"}, http.StatusOK, nil)
	if tickets := archivedIDs(t, handler, "alpha"); len(tickets) != 1 || tickets[0].ID != "AL-8" {
		t.Errorf("archived after delete = %+v", tickets)
	}
	if detail := ticketDetail(t, handler, "BE-1"); len(detail.BlockedBy) != 0 {
		t.Errorf("BE-1 still blocked: %+v", detail.BlockedBy)
	}
	getJSON(t, handler, "/api/projects/alpha/archive/AL-3", http.StatusNotFound, nil)
}

func TestLiveTicketsCannotBeDeleted(t *testing.T) {
	t.Parallel()

	handler, _ := writableAPI(t)
	getJSON(t, handler, "/api/projects/alpha/archive/AL-2", http.StatusNotFound, nil)
	send(t, handler, http.MethodPost, "/api/projects/alpha/archive/AL-2/delete", DeleteRequest{Token: "x", Confirm: "AL-2"}, http.StatusNotFound, nil)
	send(t, handler, http.MethodPost, "/api/projects/alpha/archive/..%2Ftickets/delete", DeleteRequest{Token: "x", Confirm: "../tickets"}, http.StatusBadRequest, nil)
	if detail := ticketDetail(t, handler, "AL-2"); detail.ID != "AL-2" {
		t.Errorf("AL-2 = %+v", detail)
	}

	readOnly, _ := sampleAPI(t, nil)
	send(t, readOnly, http.MethodPost, "/api/projects/alpha/archive/AL-8/delete", DeleteRequest{Token: "x", Confirm: "AL-8"}, http.StatusNotImplemented, nil)
}
