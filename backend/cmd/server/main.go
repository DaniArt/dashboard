package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/api"
	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/candidates"
	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/config"
)

func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting QA Dashboard server on port %d", cfg.Server.Port)
	log.Printf("Poll interval: %v", cfg.Server.PollInterval)
	log.Printf("Monitoring %d employees", len(cfg.Employees))

	handler := api.NewHandler(cfg)
	// Database connection for candidates
	dbDSN := os.Getenv("DADH_DSN")
	if dbDSN == "" {
		dbDSN = "postgres://qa:secure_password@10.0.99.188:5432/qa_dashboard?sslmode=disable"
	}
	var candidatesHandler *candidates.Handler
	if dbDSN != "none" {
		candidatesHandler = candidates.NewHandlerWithDB(dbDSN)
	} else {
		candidatesHandler = candidates.NewHandler("data/candidates.json")
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	candidatesHandler.RegisterRoutes(mux)

	// Serve frontend static files with no-cache headers
	fsHandler := http.FileServer(http.Dir("../frontend"))
	mux.Handle("/", noCacheMiddleware(fsHandler))
	// Also serve under /qa-dashboard/ for local testing (matches <base href>)
	mux.Handle("/qa-dashboard/", noCacheMiddleware(http.StripPrefix("/qa-dashboard/", fsHandler)))

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}
