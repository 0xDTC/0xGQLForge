package server

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/0xdtc/graphscope/internal/handler"
	"github.com/0xdtc/graphscope/internal/storage"
)

// Config holds server configuration.
type Config struct {
	Addr       string
	ProxyAddr  string
	StaticFS   embed.FS
	TemplateFS embed.FS
}

// Server is the main web server for GraphScope.
type Server struct {
	cfg      Config
	db       *storage.DB
	handlers *handler.Handlers
	tmpl     *template.Template
	httpSrv  *http.Server
}

// New creates a new Server with the given config and embedded filesystems.
func New(cfg Config, db *storage.DB, handlers *handler.Handlers) (*Server, error) {
	s := &Server{
		cfg:      cfg,
		db:       db,
		handlers: handlers,
	}

	if err := s.parseTemplates(); err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	handlers.SetTemplates(s.tmpl)

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpSrv = &http.Server{
		Addr:         cfg.Addr,
		Handler:      chain(mux, recovery, logging),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s, nil
}

// parseTemplates loads all HTML templates from the embedded filesystem.
func (s *Server) parseTemplates() error {
	funcMap := template.FuncMap{
		"json": handler.ToJSON,
		"add":  func(a, b int) int { return a + b },
		"sub":  func(a, b int) int { return a - b },
	}

	tmpl := template.New("").Funcs(funcMap)

	// Walk embedded template FS and parse all .html files
	err := fs.WalkDir(s.cfg.TemplateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || len(path) < 5 || path[len(path)-5:] != ".html" {
			return nil
		}
		data, err := fs.ReadFile(s.cfg.TemplateFS, path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", path, err)
		}
		// Use path relative to templates dir as template name
		name := path[len("templates/"):]
		if _, err := tmpl.New(name).Parse(string(data)); err != nil {
			return fmt.Errorf("parse template %s: %w", name, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	s.tmpl = tmpl
	return nil
}

// Start begins serving HTTP requests.
func (s *Server) Start() error {
	log.Printf("GraphScope web UI: http://%s", s.cfg.Addr)
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}
