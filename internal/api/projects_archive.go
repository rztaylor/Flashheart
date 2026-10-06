package api

import (
	"net/http"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// Archived projects (PRJ-5). Archiving moves a project to <root>/.archive/
// after the UI has shown what is live in it (GET .../archive-check);
// restoring moves it back unless a live project has its name. A project is
// deleted permanently only once archived, with its name typed and the token
// of the preview at GET /api/archived-projects/{project}. Agents have no
// archive or delete tool.

// ProjectArchive is the store's project archive side.
type ProjectArchive interface {
	ArchiveProject(project string) error
	RestoreProject(project string) error
	PlanProjectDelete(project string) (store.ProjectDeletePlan, error)
	DeleteArchivedProject(project, token string) error
}

// ArchiveCheckJSON is GET /api/projects/{project}/archive-check: what is in
// the project and what is live, shown before it is archived.
type ArchiveCheckJSON struct {
	Name          string         `json:"name"`
	DisplayName   string         `json:"displayName"`
	Tickets       int            `json:"tickets"`
	Counts        map[string]int `json:"counts"`
	LiveRuns      int            `json:"liveRuns"`
	Claimed       int            `json:"claimed"`
	OpenQuestions int            `json:"openQuestions"`
}

// ArchivedProjectJSON is one archived project.
type ArchivedProjectJSON struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Key         string   `json:"key"`
	Repos       []string `json:"repos"`
	Tickets     int      `json:"tickets"`
	Archived    string   `json:"archived"`
}

// ArchivedProjectsResponse is GET /api/archived-projects.
type ArchivedProjectsResponse struct {
	Revision uint64                `json:"revision"`
	Projects []ArchivedProjectJSON `json:"projects"`
}

// ProjectReferenceJSON is a ticket in another project that depends on the
// project being deleted.
type ProjectReferenceJSON struct {
	Project   string   `json:"project"`
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	DependsOn []string `json:"dependsOn"`
}

// ProjectDeletePlanJSON is GET /api/archived-projects/{project}.
type ProjectDeletePlanJSON struct {
	Name        string                 `json:"name"`
	DisplayName string                 `json:"displayName"`
	Key         string                 `json:"key"`
	Tickets     int                    `json:"tickets"`
	References  []ProjectReferenceJSON `json:"references"`
	Token       string                 `json:"token"`
}

func (b boardAPI) registerProjectArchive(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/projects/{project}/archive-check", b.archiveCheck)
	mux.HandleFunc("POST /api/projects/{project}/archive-project", b.archiveProject)
	mux.HandleFunc("GET /api/archived-projects", b.archivedProjects)
	mux.HandleFunc("GET /api/archived-projects/{project}", b.projectDeletePlan)
	mux.HandleFunc("POST /api/archived-projects/{project}/restore", b.restoreProject)
	mux.HandleFunc("POST /api/archived-projects/{project}/delete", b.deleteProject)
}

// projectArchive returns the project archive side, or answers 501.
func (b boardAPI) projectArchive(w http.ResponseWriter) ProjectArchive {
	archive, ok := b.write.(ProjectArchive)
	if b.write == nil || !ok {
		writeError(w, http.StatusNotImplemented, "read_only", "This server does not archive projects")
		return nil
	}
	return archive
}

func (b boardAPI) archiveCheck(w http.ResponseWriter, r *http.Request) {
	snapshot, project := b.project(w, r)
	if project == nil {
		return
	}
	check := ArchiveCheckJSON{
		Name: project.Name, DisplayName: project.DisplayName, Tickets: len(project.Tickets),
		Counts: map[string]int{}, LiveRuns: snapshot.RunCounts(project.Name).Live,
	}
	for _, column := range board.Columns {
		check.Counts[string(column)] = 0
	}
	for _, ticket := range project.Tickets {
		check.Counts[string(ticket.Column)]++
		if _, ok := snapshot.TicketRun(ticket.ID); ok {
			check.Claimed++
		}
	}
	for _, run := range snapshot.Runs {
		if run.Project != project.Name || run.State == runs.Ended {
			continue
		}
		for _, question := range run.Questions {
			if question.Open() {
				check.OpenQuestions++
			}
		}
	}
	writeJSON(w, http.StatusOK, check)
}

func (b boardAPI) archiveProject(w http.ResponseWriter, r *http.Request) {
	archive := b.projectArchive(w)
	if archive == nil {
		return
	}
	_, project := b.project(w, r)
	if project == nil {
		return
	}
	if err := archive.ArchiveProject(project.Name); err != nil {
		writeStoreError(w, err)
		return
	}
	b.saved(w, http.StatusOK, "", nil)
}

func archivedProjectJSON(project board.ArchivedProject) ArchivedProjectJSON {
	return ArchivedProjectJSON{
		Name: project.Name, DisplayName: project.DisplayName, Key: project.Key,
		Repos: nonNil(project.Repos), Tickets: len(project.IDs), Archived: timestamp(project.Archived),
	}
}

func (b boardAPI) archivedProjects(w http.ResponseWriter, _ *http.Request) {
	snapshot := b.snapshot(w)
	if snapshot == nil {
		return
	}
	response := ArchivedProjectsResponse{Revision: snapshot.Revision, Projects: []ArchivedProjectJSON{}}
	for _, project := range snapshot.Board.ArchivedProjects {
		response.Projects = append(response.Projects, archivedProjectJSON(project))
	}
	slices.SortFunc(response.Projects, func(a, c ArchivedProjectJSON) int {
		return strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(c.DisplayName))
	})
	writeJSON(w, http.StatusOK, response)
}

// archivedName reads {project}, refusing anything but a project name.
func archivedName(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.PathValue("project")
	if !store.ValidProject(name) {
		writeError(w, http.StatusBadRequest, "invalid_input", "Give an archived project's name")
		return "", false
	}
	return name, true
}

func (b boardAPI) projectDeletePlan(w http.ResponseWriter, r *http.Request) {
	name, ok := archivedName(w, r)
	if !ok {
		return
	}
	archive := b.projectArchive(w)
	if archive == nil {
		return
	}
	plan, err := archive.PlanProjectDelete(name)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	response := ProjectDeletePlanJSON{
		Name: plan.Name, DisplayName: plan.DisplayName, Key: plan.Key, Tickets: plan.Tickets,
		References: []ProjectReferenceJSON{}, Token: plan.Token,
	}
	for _, reference := range plan.References {
		response.References = append(response.References, ProjectReferenceJSON{
			Project: reference.Project, ID: reference.ID, Title: reference.Title, DependsOn: reference.DependsOn,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func (b boardAPI) restoreProject(w http.ResponseWriter, r *http.Request) {
	name, ok := archivedName(w, r)
	if !ok {
		return
	}
	archive := b.projectArchive(w)
	if archive == nil {
		return
	}
	if err := archive.RestoreProject(name); err != nil {
		writeStoreError(w, err)
		return
	}
	b.saved(w, http.StatusOK, "", nil)
}

func (b boardAPI) deleteProject(w http.ResponseWriter, r *http.Request) {
	name, ok := archivedName(w, r)
	if !ok {
		return
	}
	archive := b.projectArchive(w)
	if archive == nil {
		return
	}
	var request DeleteRequest
	if !decode(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.Confirm) != name {
		writeError(w, http.StatusBadRequest, "invalid_input", "Type the project's name to delete it permanently")
		return
	}
	if err := archive.DeleteArchivedProject(name, request.Token); err != nil {
		writeStoreError(w, err)
		return
	}
	b.saved(w, http.StatusOK, "", nil)
}
