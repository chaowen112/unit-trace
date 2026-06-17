package main

import (
	"context"
	_ "embed"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"unittrace/api"
	"unittrace/imgworker"
	"unittrace/matching"
	"unittrace/store"
)

//go:embed schema.sql
var schemaSQL string

func main() {
	cfg := loadConfig()

	// Create image directory
	if err := os.MkdirAll(cfg.ImageDir, 0755); err != nil {
		log.Fatalf("failed to create image directory %s: %v", cfg.ImageDir, err)
	}

	ctx := context.Background()

	// Connect to DB and apply schema
	st, err := store.New(ctx, cfg.DatabaseURL, schemaSQL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer st.Close()

	log.Printf("connected to database")

	// Set up matching engine
	engine := matching.NewEngine(st)

	// Set up image worker
	worker, err := imgworker.NewWorker(cfg.ImageDir, st)
	if err != nil {
		log.Fatalf("failed to create image worker: %v", err)
	}

	// Set up API server
	srv := api.NewServer(st, engine, worker, cfg.ImageDir)
	handler := srv.Handler()

	httpServer := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("server listening on %s", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	log.Println("server stopped")
}
