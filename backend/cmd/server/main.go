package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/config"
	"github.com/kaskol10/org-cost-api/backend/internal/demo"
	"github.com/kaskol10/org-cost-api/backend/internal/handlers"
	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 120 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 15 * time.Second
)

func main() {
	configPath := flag.String("config", "", "path to YAML config (auto-discovered if omitted)")
	flag.Parse()

	cfg, resolvedPath, err := config.LoadAuto(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	log.Printf("using config %s", resolvedPath)

	var agg *service.Aggregator
	if cfg.Demo {
		log.Print("DEMO MODE — fixture data only, no AWS calls")
		agg, err = demo.NewAggregator(cfg)
	} else {
		agg, err = service.NewAggregator(cfg)
	}
	if err != nil {
		log.Fatalf("aggregator: %v", err)
	}

	if !cfg.Demo && agg.SnapshotCount() == 0 {
		log.Print("No cost history yet — run the dashboard once daily (or call get_org_summary) to enable free prior-period trends.")
	}

	apiToken := ""
	if !cfg.Demo {
		apiToken = cfg.ResolveAPIToken()
		if apiToken != "" {
			log.Print("API bearer token auth enabled for /api/* routes (except /api/health and /api/ready)")
		}
	}

	staticDir := os.Getenv("STATIC_DIR")
	api := handlers.New(agg, cfg.CORSOrigin, apiToken)
	handler := api.Routes(staticDir)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	go func() {
		log.Printf("listening on %s", cfg.ListenAddr)
		if staticDir != "" {
			log.Printf("serving static files from %s", staticDir)
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Print("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Print("server stopped")
}
