package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/0xDTC/0xGQLForge/internal/analysis"
)

// AnalysisView renders the security analysis page.
func (h *Handlers) AnalysisView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.SchemaRepo.Get(id)
	if err != nil || s == nil {
		http.Error(w, "Schema not found", http.StatusNotFound)
		return
	}

	data := map[string]any{
		"Title":  "Analysis: " + s.Name,
		"Schema": s,
	}
	h.render(w, "analysis.html", data)
}

// RunAnalysis handles POST /api/analysis/run — runs all security analyses on a schema.
func (h *Handlers) RunAnalysis(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SchemaID string   `json:"schemaId"`
		Modules  []string `json:"modules"` // optional: specific modules to run
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	s, err := h.SchemaRepo.Get(req.SchemaID)
	if err != nil || s == nil {
		jsonErr(w, http.StatusNotFound, "schema not found")
		return
	}

	results := analysis.RunAll(s)

	// Persist results
	for aType, result := range results {
		resultJSON, err := json.Marshal(result)
		if err != nil {
			log.Printf("marshal analysis result %s: %v", aType, err)
			continue
		}
		id := generateID()
		if err := h.AnalysisRepo.Save(id, req.SchemaID, aType, string(resultJSON)); err != nil {
			log.Printf("save analysis result %s: %v", aType, err)
		}
	}

	jsonResp(w, http.StatusOK, results)
}

// AnalysisResults returns stored analysis results for a schema.
func (h *Handlers) AnalysisResults(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	stored, err := h.AnalysisRepo.ListBySchema(id)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Parse stored JSON back into structured results
	results := make(map[string]any)
	for aType, resultJSON := range stored {
		var v any
		if err := json.Unmarshal([]byte(resultJSON), &v); err != nil {
			log.Printf("unmarshal analysis result %s: %v", aType, err)
			continue
		}
		results[aType] = v
	}

	jsonResp(w, http.StatusOK, results)
}

// FuzzFields handles POST /api/fuzz — runs field fuzzing against a target.
func (h *Handlers) FuzzFields(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetURL string   `json:"targetUrl"`
		TypeName  string   `json:"typeName"`
		Words     []string `json:"words"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}

	result := analysis.FuzzFields(req.TargetURL, req.TypeName, req.Words)
	jsonResp(w, http.StatusOK, result)
}

// BypassIntrospection handles POST /api/bypass — tries introspection bypass techniques.
func (h *Handlers) BypassIntrospection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetURL string `json:"targetUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}

	results := analysis.TryBypass(req.TargetURL)
	jsonResp(w, http.StatusOK, results)
}

