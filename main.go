package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"callprocessor/handlers"
	"callprocessor/persistence"
)

func main() {
	// Config comes from env vars so the same image runs in Compose, Kubernetes, etc.
	port := getEnv("PORT", "8080")
	mongoURI := getEnv("MONGODB_URI", "mongodb://localhost:27017")
	database := getEnv("MONGODB_DATABASE", "callrail")

	if err := persistence.Connect(mongoURI, database); err != nil {
		log.Fatalf("failed to configure MongoDB client: %v", err)
	}

	indexCtx, cancelIndex := context.WithTimeout(context.Background(), 30*time.Second)
	if err := persistence.EnsureIndexes(indexCtx); err != nil {
		log.Fatalf("failed to ensure MongoDB indexes: %v", err)
	}
	cancelIndex()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/calls", handlers.ListCalls)
	mux.HandleFunc("POST /api/calls", handlers.CreateCall)
	mux.HandleFunc("GET /healthz", handlers.Healthz)
	mux.HandleFunc("GET /readyz", handlers.Readyz)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Kubernetes sends SIGTERM before killing a pod; Ctrl+C sends SIGINT.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Server starting on port %s...", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")

	// Let in-flight requests finish before closing the Mongo connection pool.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}
	if err := persistence.Disconnect(shutdownCtx); err != nil {
		log.Printf("MongoDB disconnect error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
