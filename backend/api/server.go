package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"unittrace/imgworker"
	"unittrace/matching"
	"unittrace/store"
)

// Server holds the application dependencies.
type Server struct {
	store       *store.Store
	matcher     *matching.Engine
	imageWorker *imgworker.Worker
	imageDir    string
}

// NewServer creates a new API server.
func NewServer(st *store.Store, matcher *matching.Engine, worker *imgworker.Worker, imageDir string) *Server {
	return &Server{
		store:       st,
		matcher:     matcher,
		imageWorker: worker,
		imageDir:    imageDir,
	}
}

// Handler sets up and returns the HTTP handler with all routes.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	// Dashboard (HTML)
	r.Get("/", s.handleDashboard)
	r.Get("/dashboard", s.handleDashboard)

	// API (JSON)
	r.Group(func(r chi.Router) {
		r.Use(middleware.SetHeader("Content-Type", "application/json"))

		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		r.Post("/api/v1/listing-views", s.handleSubmitListingView)
		r.Get("/api/v1/tracked-units", s.handleListTrackedUnits)
		r.Get("/api/v1/tracked-units/{id}", s.handleGetTrackedUnit)
		r.Post("/api/v1/tracked-units/{id}/notes", s.handleAddNote)
		r.Post("/api/v1/listing-status-batch", s.handleBatchListingStatus)
		r.Get("/api/v1/images/{sha256}", s.handleServeImage)
	})

	return r
}

// corsMiddleware adds CORS headers for all requests.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleServeImage serves an image file from the image directory by sha256.
func (s *Server) handleServeImage(w http.ResponseWriter, r *http.Request) {
	sha256 := chi.URLParam(r, "sha256")
	if sha256 == "" || len(sha256) < 2 {
		http.Error(w, "invalid sha256", http.StatusBadRequest)
		return
	}

	// Try common extensions
	exts := []string{".jpg", ".png", ".gif", ".webp", ".bin"}
	for _, ext := range exts {
		path := filepath.Join(s.imageDir, sha256[:2], sha256+ext)
		http.ServeFile(w, r, path)
		return
	}
	http.NotFound(w, r)
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Can't do much if encoding fails after WriteHeader
		return
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
