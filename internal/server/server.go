package server

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/0xDTC/0xGQLForge/internal/handler"
	"github.com/0xDTC/0xGQLForge/internal/storage"
)

// Config holds server configuration.
type Config struct {
	Addr       string
	ProxyAddr  string
	StaticFS   embed.FS
	TemplateFS embed.FS
}

// Server is the main web server for 0xGQLForge.
type Server struct {
	cfg      Config
	db       *storage.DB
	handlers *handler.Handlers
	tmpls    map[string]*template.Template
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

	handlers.SetTemplates(s.tmpls)

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
// Each page gets its own template set (layout + page) so that multiple
// templates can each define a "content" block without colliding.
func (s *Server) parseTemplates() error {
	funcMap := template.FuncMap{
		"json": handler.ToJSON,
		"add":  func(a, b int) int { return a + b },
		"sub":  func(a, b int) int { return a - b },
	}

	// Read the layout template
	layoutData, err := fs.ReadFile(s.cfg.TemplateFS, "templates/layout.html")
	if err != nil {
		return fmt.Errorf("read layout: %w", err)
	}

	s.tmpls = make(map[string]*template.Template)

	// Walk all template files
	err = fs.WalkDir(s.cfg.TemplateFS, "templates", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || len(path) < 5 || path[len(path)-5:] != ".html" {
			return nil
		}

		name := path[len("templates/"):]
		if name == "layout.html" {
			return nil
		}

		data, err := fs.ReadFile(s.cfg.TemplateFS, path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", path, err)
		}

		if strings.HasPrefix(name, "partials/") {
			// Partials are standalone fragments (no layout wrapper)
			t, err := template.New(name).Funcs(funcMap).Parse(string(data))
			if err != nil {
				return fmt.Errorf("parse partial %s: %w", name, err)
			}
			s.tmpls[name] = t
		} else {
			// Page templates: clone layout, then parse the page into it
			t, err := template.New("layout.html").Funcs(funcMap).Parse(string(layoutData))
			if err != nil {
				return fmt.Errorf("parse layout for %s: %w", name, err)
			}
			if _, err := t.New(name).Parse(string(data)); err != nil {
				return fmt.Errorf("parse template %s: %w", name, err)
			}
			s.tmpls[name] = t
		}

		return nil
	})
	return err
}

// Start begins serving HTTP requests.
func (s *Server) Start() error {
	log.Printf("0xGQLForge web UI: http://%s", s.cfg.Addr)
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}
