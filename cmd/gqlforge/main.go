package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/0xDTC/0xGQLForge/internal/handler"
	"github.com/0xDTC/0xGQLForge/internal/proxy"
	"github.com/0xDTC/0xGQLForge/internal/server"
	"github.com/0xDTC/0xGQLForge/internal/storage"
	"github.com/0xDTC/0xGQLForge/web"
)

func main() {
	addr := flag.String("addr", ":8080", "Web UI listen address")
	proxyAddr := flag.String("proxy", ":8888", "MITM proxy listen address")
	dbPath := flag.String("db", "", "SQLite database path (default: ~/.gqlforge/gqlforge.db)")
	autoProxy := flag.Bool("auto-proxy", false, "Start proxy automatically on launch")
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lshortfile)

	// Config directory
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("get home dir: %v", err)
	}
	configDir := filepath.Join(home, ".gqlforge")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		log.Fatalf("create config dir: %v", err)
	}

	// Database
	db, err := storage.New(*dbPath)
	if err != nil {
		log.Fatalf("init database: %v", err)
	}
	defer db.Close()

	// Repos
	schemaRepo := storage.NewSchemaRepo(db)
	trafficRepo := storage.NewTrafficRepo(db)
	analysisRepo := storage.NewAnalysisRepo(db)
	projectRepo := storage.NewProjectRepo(db)

	// Handlers
	handlers := handler.NewHandlers(schemaRepo, trafficRepo, analysisRepo, projectRepo)

	// Certificate manager
	certMgr, err := proxy.NewCertManager(configDir)
	if err != nil {
		log.Fatalf("init cert manager: %v", err)
	}

	// Proxy
	p := proxy.NewProxy(*proxyAddr, certMgr, trafficRepo)
	handlers.SetProxyController(p)

	if *autoProxy {
		if err := p.Start(); err != nil {
			log.Printf("WARNING: failed to auto-start proxy: %v", err)
		}
	}

	// Server
	srv, err := server.New(server.Config{
		Addr:       *addr,
		ProxyAddr:  *proxyAddr,
		StaticFS:   web.StaticFS,
		TemplateFS: web.TemplateFS,
	}, db, handlers)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		fmt.Println("\nShutting down...")
		p.Stop()
		srv.Shutdown(context.Background())
		db.Close()
		os.Exit(0)
	}()

	fmt.Print(`
   ___        ____  ___  _     _____
  / _ \__  __/ ___|/ _ \| |   |  ___|__  _ __ __ _  ___
 | | | \ \/ / |  _| | | | |   | |_ / _ \| '__/ _` + "`" + ` |/ _ \
 | |_| |>  <| |_| | |_| | |___|  _| (_) | | | (_| |  __/
  \___//_/\_\\____|\___ \|_____|_|  \___/|_|  \__, |\___|
                       |_|                    |___/
`)
	fmt.Printf("  Web UI:    http://localhost%s\n", *addr)
	fmt.Printf("  Proxy:     %s\n", *proxyAddr)
	fmt.Printf("  CA Cert:   %s\n", certMgr.CACertPath())
	fmt.Printf("  Database:  %s\n", filepath.Join(configDir, "gqlforge.db"))
	fmt.Println()

	if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
		log.Fatalf("server error: %v", err)
	}
}
