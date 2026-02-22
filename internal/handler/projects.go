package handler

import (
	"log"
	"net/http"

	"github.com/0xDTC/0xGQLForge/internal/inference"
	"github.com/0xDTC/0xGQLForge/internal/schema"
)

// ProjectsList renders the projects overview page.
func (h *Handlers) ProjectsList(w http.ResponseWriter, r *http.Request) {
	projects, _ := h.ProjectRepo.List()
	data := map[string]any{
		"Title":    "Projects",
		"Projects": projects,
	}
	h.render(w, "projects.html", data)
}

// ProjectDetail renders the detail page for a single project.
func (h *Handlers) ProjectDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, err := h.ProjectRepo.Get(id)
	if err != nil || project == nil {
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	traffic, _ := h.TrafficRepo.ListByProject(id, 500)

	var s *schema.Schema
	if project.SchemaID != nil {
		s, _ = h.SchemaRepo.Get(*project.SchemaID)
	}

	data := map[string]any{
		"Title":   "Project: " + project.Name,
		"Project": project,
		"Traffic": traffic,
		"Schema":  s,
	}
	h.render(w, "project_detail.html", data)
}

// ProjectInferSchema runs schema inference on the project's captured traffic
// and saves the result as a new schema, linking it to the project.
func (h *Handlers) ProjectInferSchema(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, err := h.ProjectRepo.Get(id)
	if err != nil || project == nil {
		jsonErr(w, http.StatusNotFound, "project not found")
		return
	}

	traffic, err := h.TrafficRepo.ListByProject(id, 0)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(traffic) == 0 {
		jsonErr(w, http.StatusBadRequest, "no traffic captured for this project")
		return
	}

	s := inference.BuildFromTraffic(traffic, project.Name)

	if err := h.SchemaRepo.Save(s, "{}"); err != nil {
		jsonErr(w, http.StatusInternalServerError, "save schema: "+err.Error())
		return
	}

	if err := h.ProjectRepo.UpdateSchema(id, s.ID); err != nil {
		log.Printf("update project schema: %v", err)
	}

	jsonResp(w, http.StatusOK, map[string]any{
		"schemaId":    s.ID,
		"typeCount":   len(s.Types),
		"redirectURL": "/schema/" + s.ID,
	})
}
