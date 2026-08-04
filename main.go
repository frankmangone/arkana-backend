package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"arkana/config"
	"arkana/router"
	"arkana/shared/rediscache"

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

	// Initialize Redis (ephemeral quiz attempt/session state)
	redisClient, err := rediscache.NewClient(cfg.RedisAddr)
	if err != nil {
		log.Fatal("Failed to initialize Redis client:", err)
	}
	defer redisClient.Close()

	// Run migrations
	if err := goose.SetDialect("sqlite3"); err != nil {
		log.Fatal("Failed to set migration dialect:", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Setup router with all routes
	r := router.Setup(db, cfg, redisClient)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.ApiPort),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Println(fmt.Sprintf("Server listening on :%s", cfg.ApiPort))
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
