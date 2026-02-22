package handler

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/0xDTC/0xGQLForge/internal/storage"
)

// Handlers holds all HTTP handler dependencies.
type Handlers struct {
	SchemaRepo     *storage.SchemaRepo
	TrafficRepo    *storage.TrafficRepo
	AnalysisRepo   *storage.AnalysisRepo
	ProjectRepo    *storage.ProjectRepo
	tmpls          map[string]*template.Template
	proxyCtrl      ProxyController
	currentProject string // label for the active proxy session
}

// ProxyController is the interface the handlers use to control the proxy.
type ProxyController interface {
	Start() error
	Stop() error
	Running() bool
	Addr() string
	Subscribe() <-chan []byte
	Unsubscribe(<-chan []byte)
	SetProjectID(string)
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(sr *storage.SchemaRepo, tr *storage.TrafficRepo, ar *storage.AnalysisRepo, pr *storage.ProjectRepo) *Handlers {
	return &Handlers{
		SchemaRepo:   sr,
		TrafficRepo:  tr,
		AnalysisRepo: ar,
		ProjectRepo:  pr,
	}
}

// SetTemplates sets the parsed template collection for page rendering.
func (h *Handlers) SetTemplates(tmpls map[string]*template.Template) {
	h.tmpls = tmpls
}

// SetProxyController wires the proxy control interface.
func (h *Handlers) SetProxyController(ctrl ProxyController) {
	h.proxyCtrl = ctrl
}

// render executes a named template with the given data.
// Page templates are executed via "layout.html"; partials are executed directly.
func (h *Handlers) render(w http.ResponseWriter, name string, data any) {
	t, ok := h.tmpls[name]
	if !ok {
		http.Error(w, "Template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Page templates are rooted at "layout.html"; partials use their own name.
	execName := "layout.html"
	if strings.HasPrefix(name, "partials/") {
		execName = name
	}
	if err := t.ExecuteTemplate(w, execName, data); err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// jsonResp writes a JSON response with the given status code.
func jsonResp(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// jsonErr writes a JSON error response.
func jsonErr(w http.ResponseWriter, code int, msg string) {
	jsonResp(w, code, map[string]string{"error": msg})
}

// ToJSON is a template helper that marshals data to JSON string.
func ToJSON(v any) template.JS {
	b, _ := json.Marshal(v)
	return template.JS(b)
}
