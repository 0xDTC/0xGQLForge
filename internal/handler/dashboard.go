package handler

import (
	"log"
	"net/http"
)

// SchemasList renders the schemas management page (home page).
func (h *Handlers) SchemasList(w http.ResponseWriter, r *http.Request) {
	schemas, err := h.SchemaRepo.List()
	if err != nil {
		log.Printf("list schemas: %v", err)
		http.Error(w, "Failed to load schemas", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":   "Schemas",
		"Schemas": schemas,
	}
	h.render(w, "schemas.html", data)
}
