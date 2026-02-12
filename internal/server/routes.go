package server

import (
	"io/fs"
	"net/http"
)

// registerRoutes sets up all HTTP routes on the given mux.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	h := s.handlers

	// Static files
	staticFS, err := fs.Sub(s.cfg.StaticFS, "static")
	if err == nil {
		mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	}

	// Pages
	mux.HandleFunc("GET /{$}", h.Dashboard)
	mux.HandleFunc("GET /schema/{id}", h.SchemaView)
	mux.HandleFunc("GET /schema/{id}/graph", h.SchemaGraph)
	mux.HandleFunc("GET /generator/{id}", h.GeneratorView)
	mux.HandleFunc("GET /proxy", h.ProxyView)
	mux.HandleFunc("GET /analysis/{id}", h.AnalysisView)

	// API — Introspection
	mux.HandleFunc("POST /api/introspection", h.IntrospectionParse)

	// API — Schema
	mux.HandleFunc("GET /api/schemas", h.SchemaList)
	mux.HandleFunc("GET /api/schema/{id}", h.SchemaDetail)
	mux.HandleFunc("GET /api/schema/{id}/graph-data", h.SchemaGraphData)
	mux.HandleFunc("GET /api/schema/{id}/operations", h.SchemaOperations)
	mux.HandleFunc("DELETE /api/schema/{id}", h.SchemaDelete)

	// API — Query Generator
	mux.HandleFunc("POST /api/generate", h.GenerateQuery)

	// API — Proxy
	mux.HandleFunc("GET /api/proxy/traffic", h.ProxyTraffic)
	mux.HandleFunc("POST /api/proxy/start", h.ProxyStart)
	mux.HandleFunc("POST /api/proxy/stop", h.ProxyStop)
	mux.HandleFunc("GET /api/proxy/status", h.ProxyStatus)
	mux.HandleFunc("DELETE /api/proxy/traffic", h.ProxyClearTraffic)
	mux.HandleFunc("GET /api/proxy/sse", h.ProxySSE)

	// API — Analysis
	mux.HandleFunc("POST /api/analysis/run", h.RunAnalysis)
	mux.HandleFunc("GET /api/analysis/{id}", h.AnalysisResults)

	// API — Similarity
	mux.HandleFunc("GET /api/similarity/clusters", h.SimilarityClusters)

	// API — Fuzzer
	mux.HandleFunc("POST /api/fuzz", h.FuzzFields)

	// API — Bypass
	mux.HandleFunc("POST /api/bypass", h.BypassIntrospection)

	// API — Diff
	mux.HandleFunc("POST /api/diff", h.DiffSchemas)

	// HTMX Partials
	mux.HandleFunc("GET /partial/type/{schemaID}/{typeName}", h.PartialTypeDetail)
	mux.HandleFunc("GET /partial/operation/{schemaID}/{opName}", h.PartialOperationDetail)
	mux.HandleFunc("GET /partial/traffic/{id}", h.PartialTrafficDetail)
}
