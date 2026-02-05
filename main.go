package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"arkana/config"
	"arkana/router"

	"github.com/pressly/goose/v3"
)

func main() {
	log.Println("Starting server...")

	// Load and validate configuration
	cfg, err := config.LoadAndValidate()
	if err != nil {
		log.Fatal("Configuration error:", err)
	}

	// Initialize database
	db, err := initDB(cfg.DatabasePath)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Run migrations
	goose.SetDialect("sqlite3")
	if err := goose.Up(db, "migrations"); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Setup router with all routes
	r := router.Setup(db, cfg.CORSAllowedOrigin)

	srv := &http.Server{
		Addr:    ":8082",
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		log.Println("Server listening on :8082")
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server stopped")
}
