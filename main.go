package main

import (
	"log"
	"net/http"

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
	router := router.Setup(db, cfg)

	log.Println("Server starting on :8082")
	log.Fatal(http.ListenAndServe(":8082", router))
}
