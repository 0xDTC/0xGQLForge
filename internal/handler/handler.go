package handler

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/0xdtc/graphscope/internal/storage"
)

// Handlers holds all HTTP handler dependencies.
type Handlers struct {
	SchemaRepo   *storage.SchemaRepo
	TrafficRepo  *storage.TrafficRepo
	AnalysisRepo *storage.AnalysisRepo
	tmpl         *template.Template
	proxyCtrl    ProxyController
}

// ProxyController is the interface the handlers use to control the proxy.
type ProxyController interface {
	Start() error
	Stop() error
	Running() bool
	Addr() string
	Subscribe() <-chan []byte
	Unsubscribe(<-chan []byte)
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(sr *storage.SchemaRepo, tr *storage.TrafficRepo, ar *storage.AnalysisRepo) *Handlers {
	return &Handlers{
		SchemaRepo:   sr,
		TrafficRepo:  tr,
		AnalysisRepo: ar,
	}
}

// SetTemplates sets the parsed template collection for page rendering.
func (h *Handlers) SetTemplates(tmpl *template.Template) {
	h.tmpl = tmpl
}

// SetProxyController wires the proxy control interface.
func (h *Handlers) SetProxyController(ctrl ProxyController) {
	h.proxyCtrl = ctrl
}

// render executes a named template with the given data.
func (h *Handlers) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
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
