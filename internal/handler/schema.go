package handler

import (
	"net/http"

	"github.com/0xdtc/graphscope/internal/schema"
)

// SchemaView renders the schema explorer page.
func (h *Handlers) SchemaView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.SchemaRepo.Get(id)
	if err != nil || s == nil {
		http.Error(w, "Schema not found", http.StatusNotFound)
		return
	}

	ops := schema.GetOperations(s)
	userTypes := schema.UserTypes(s)
	byKind := schema.TypesByKind(s)

	data := map[string]any{
		"Title":        "Schema: " + s.Name,
		"Schema":       s,
		"Operations":   ops,
		"UserTypes":    userTypes,
		"TypesByKind":  byKind,
		"QueryOps":     filterOps(ops, "query"),
		"MutationOps":  filterOps(ops, "mutation"),
		"SubOps":       filterOps(ops, "subscription"),
		"Objects":      byKind[schema.KindObject],
		"InputObjects": byKind[schema.KindInputObject],
		"Enums":        byKind[schema.KindEnum],
		"Interfaces":   byKind[schema.KindInterface],
		"Unions":       byKind[schema.KindUnion],
		"Scalars":      byKind[schema.KindScalar],
	}
	h.render(w, "schema.html", data)
}

// SchemaGraph renders the D3.js graph visualization page.
func (h *Handlers) SchemaGraph(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.SchemaRepo.Get(id)
	if err != nil || s == nil {
		http.Error(w, "Schema not found", http.StatusNotFound)
		return
	}

	gd := schema.BuildGraphData(s)

	data := map[string]any{
		"Title":     "Graph: " + s.Name,
		"Schema":    s,
		"GraphData": gd,
	}
	h.render(w, "graph.html", data)
}

// SchemaList returns all schemas as JSON (API endpoint).
func (h *Handlers) SchemaList(w http.ResponseWriter, r *http.Request) {
	schemas, err := h.SchemaRepo.List()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]map[string]any, len(schemas))
	for i, s := range schemas {
		ops := schema.GetOperations(&s)
		result[i] = map[string]any{
			"id":            s.ID,
			"name":          s.Name,
			"source":        s.Source,
			"typeCount":     len(schema.UserTypes(&s)),
			"queryCount":    countOps(ops, "query"),
			"mutationCount": countOps(ops, "mutation"),
			"createdAt":     s.CreatedAt,
		}
	}
	jsonResp(w, http.StatusOK, result)
}

// SchemaDetail returns a single schema's full data as JSON.
func (h *Handlers) SchemaDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.SchemaRepo.Get(id)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s == nil {
		jsonErr(w, http.StatusNotFound, "schema not found")
		return
	}
	jsonResp(w, http.StatusOK, s)
}

// SchemaGraphData returns the D3.js graph nodes and links as JSON.
func (h *Handlers) SchemaGraphData(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.SchemaRepo.Get(id)
	if err != nil || s == nil {
		jsonErr(w, http.StatusNotFound, "schema not found")
		return
	}
	jsonResp(w, http.StatusOK, schema.BuildGraphData(s))
}

// SchemaOperations returns all operations for a schema as JSON.
func (h *Handlers) SchemaOperations(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s, err := h.SchemaRepo.Get(id)
	if err != nil || s == nil {
		jsonErr(w, http.StatusNotFound, "schema not found")
		return
	}
	jsonResp(w, http.StatusOK, schema.GetOperations(s))
}

// SchemaDelete removes a schema.
func (h *Handlers) SchemaDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.SchemaRepo.Delete(id); err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func filterOps(ops []schema.Operation, kind string) []schema.Operation {
	var result []schema.Operation
	for _, op := range ops {
		if op.Kind == kind {
			result = append(result, op)
		}
	}
	return result
}
