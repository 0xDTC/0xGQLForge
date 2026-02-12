package handler

import (
	"encoding/json"
	"net/http"

	"github.com/0xDTC/0xGQLForge/internal/generator"
	"github.com/0xDTC/0xGQLForge/internal/schema"
)

// GeneratorView renders the query generator page.
func (h *Handlers) GeneratorView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.SchemaRepo.Get(id)
	if err != nil || s == nil {
		http.Error(w, "Schema not found", http.StatusNotFound)
		return
	}

	ops := schema.GetOperations(s)
	data := map[string]any{
		"Title":       "Generator: " + s.Name,
		"Schema":      s,
		"Operations":  ops,
		"QueryOps":    filterOps(ops, "query"),
		"MutationOps": filterOps(ops, "mutation"),
		"SubOps":      filterOps(ops, "subscription"),
	}
	h.render(w, "generator.html", data)
}

// GenerateQuery handles POST /api/generate — generates a query for a specific operation.
func (h *Handlers) GenerateQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SchemaID  string `json:"schemaId"`
		Operation string `json:"operation"`
		Kind      string `json:"kind"`
		MaxDepth  int    `json:"maxDepth"`
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

	cfg := generator.DefaultConfig()
	if req.MaxDepth > 0 {
		cfg.MaxDepth = req.MaxDepth
	}

	query, variables := generator.GenerateQuery(s, req.Operation, req.Kind, cfg)
	if query == "" {
		jsonErr(w, http.StatusNotFound, "operation not found")
		return
	}

	// Also get complexity info
	complexity := generator.EstimateComplexity(s, req.Operation)

	// Find the operation to show arg details
	ops := schema.GetOperations(s)
	var opDetail *schema.Operation
	for i := range ops {
		if ops[i].Name == req.Operation && ops[i].Kind == req.Kind {
			opDetail = &ops[i]
			break
		}
	}

	jsonResp(w, http.StatusOK, map[string]any{
		"query":      query,
		"variables":  variables,
		"complexity": complexity,
		"operation":  opDetail,
	})
}
