package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/0xDTC/0xGQLForge/internal/schema"
)

// PartialTypeDetail renders a type detail panel (HTMX partial).
func (h *Handlers) PartialTypeDetail(w http.ResponseWriter, r *http.Request) {
	schemaID := r.PathValue("schemaID")
	typeName := r.PathValue("typeName")

	s, err := h.SchemaRepo.Get(schemaID)
	if err != nil || s == nil {
		http.Error(w, "Schema not found", http.StatusNotFound)
		return
	}

	t := schema.FindType(s, typeName)
	if t == nil {
		http.Error(w, "Type not found", http.StatusNotFound)
		return
	}

	data := map[string]any{
		"Schema": s,
		"Type":   t,
	}
	h.render(w, "partials/type_detail.html", data)
}

// PartialOperationDetail renders an operation detail panel (HTMX partial).
func (h *Handlers) PartialOperationDetail(w http.ResponseWriter, r *http.Request) {
	schemaID := r.PathValue("schemaID")
	opName := r.PathValue("opName")

	s, err := h.SchemaRepo.Get(schemaID)
	if err != nil || s == nil {
		http.Error(w, "Schema not found", http.StatusNotFound)
		return
	}

	ops := schema.GetOperations(s)
	var op *schema.Operation
	for i := range ops {
		if ops[i].Name == opName {
			op = &ops[i]
			break
		}
	}
	if op == nil {
		http.Error(w, "Operation not found", http.StatusNotFound)
		return
	}

	data := map[string]any{
		"Schema":    s,
		"Operation": op,
	}
	h.render(w, "partials/query_result.html", data)
}

// PartialTrafficDetail renders a traffic entry detail (HTMX partial).
func (h *Handlers) PartialTrafficDetail(w http.ResponseWriter, r *http.Request) {
	// Placeholder — will be implemented with proxy
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// SimilarityClusters returns query clusters (placeholder until similarity engine).
func (h *Handlers) SimilarityClusters(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, http.StatusOK, []any{})
}

// DiffSchemas compares two schemas (placeholder until analysis module).
func (h *Handlers) DiffSchemas(w http.ResponseWriter, r *http.Request) {
	jsonErr(w, http.StatusNotImplemented, "schema diff not yet implemented")
}

// generateID creates a random hex ID.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
