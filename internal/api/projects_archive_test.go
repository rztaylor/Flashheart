package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveRestoreAndDeleteProjects(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	var check ArchiveCheckJSON
	getJSON(t, handler, "/api/projects/alpha/archive-check", http.StatusOK, &check)
	if check.Name != "alpha" || check.Tickets != 7 || check.Counts["backlog"] != 3 {
		t.Errorf("check = %+v", check)
	}
	send(t, handler, http.MethodPost, "/api/projects/alpha/archive-project", struct{}{}, http.StatusOK, nil)
	if _, err := os.Stat(filepath.Join(root, ".archive", "alpha")); err != nil {
		t.Fatalf("not archived: %v", err)
	}
	var projects ProjectsResponse
	getJSON(t, handler, "/api/projects", http.StatusOK, &projects)
	for _, project := range projects.Projects {
		if project.Name == "alpha" {
			t.Error("alpha is still listed")
		}
	}
	if projects.ArchivedProjects != 1 {
		t.Errorf("archived projects = %d", projects.ArchivedProjects)
	}
	// Its tickets count as done for beta.
	if detail := ticketDetail(t, handler, "BE-1"); len(detail.BlockedBy) != 0 {
		t.Errorf("BE-1 blocked: %+v", detail.BlockedBy)
	}

	var list ArchivedProjectsResponse
	getJSON(t, handler, "/api/archived-projects", http.StatusOK, &list)
	if len(list.Projects) != 1 || list.Projects[0].Name != "alpha" || list.Projects[0].Key != "AL" || list.Projects[0].Tickets != 8 || list.Projects[0].Archived == "" {
		t.Fatalf("archived = %+v", list.Projects)
	}

	send(t, handler, http.MethodPost, "/api/archived-projects/alpha/restore", struct{}{}, http.StatusOK, nil)
	if _, err := os.Stat(filepath.Join(root, "alpha", "project.yaml")); err != nil {
		t.Fatalf("not restored: %v", err)
	}
	send(t, handler, http.MethodPost, "/api/archived-projects/alpha/restore", struct{}{}, http.StatusNotFound, nil)

	// Delete only from the archive, with the name typed and a fresh preview.
	send(t, handler, http.MethodPost, "/api/archived-projects/alpha/delete", DeleteRequest{Token: "x", Confirm: "alpha"}, http.StatusNotFound, nil)
	send(t, handler, http.MethodPost, "/api/projects/alpha/archive-project", struct{}{}, http.StatusOK, nil)
	var plan ProjectDeletePlanJSON
	getJSON(t, handler, "/api/archived-projects/alpha", http.StatusOK, &plan)
	if plan.Key != "AL" || plan.Tickets != 8 || len(plan.References) != 1 || plan.References[0].ID != "BE-1" || plan.Token == "" {
		t.Fatalf("plan = %+v", plan)
	}
	send(t, handler, http.MethodPost, "/api/archived-projects/alpha/delete", DeleteRequest{Token: plan.Token, Confirm: "Alpha"}, http.StatusBadRequest, nil)
	send(t, handler, http.MethodPost, "/api/archived-projects/alpha/delete", DeleteRequest{Token: "stale", Confirm: "alpha"}, http.StatusConflict, nil)
	send(t, handler, http.MethodPost, "/api/archived-projects/alpha/delete", DeleteRequest{Token: plan.Token, Confirm: "alpha"}, http.StatusOK, nil)
	if _, err := os.Stat(filepath.Join(root, ".archive", "alpha")); !os.IsNotExist(err) {
		t.Errorf("archived directory remains: %v", err)
	}
	getJSON(t, handler, "/api/archived-projects/..%2Fbeta", http.StatusBadRequest, nil)
	send(t, handler, http.MethodPost, "/api/projects/nowhere/archive-project", struct{}{}, http.StatusNotFound, nil)
}

func TestRestoreRefusesANameInUse(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	send(t, handler, http.MethodPost, "/api/projects/beta/archive-project", struct{}{}, http.StatusOK, nil)
	if err := os.MkdirAll(filepath.Join(root, "beta", "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	var conflict struct {
		Error struct{ Code, Message string }
	}
	send(t, handler, http.MethodPost, "/api/archived-projects/beta/restore", struct{}{}, http.StatusConflict, &conflict)
	if conflict.Error.Code != "exists" {
		t.Errorf("conflict = %+v", conflict)
	}
}
