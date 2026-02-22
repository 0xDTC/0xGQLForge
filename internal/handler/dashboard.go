package handler

import "net/http"

// SchemasList renders the schemas management page (home page).
func (h *Handlers) SchemasList(w http.ResponseWriter, r *http.Request) {
	schemas, _ := h.SchemaRepo.List()
	data := map[string]any{
		"Title":   "Schemas",
		"Schemas": schemas,
	}
	h.render(w, "schemas.html", data)
}
