package main

import (
	"log"
	"net/http"

	"arkana/config"
	"arkana/internal/auth"
	"arkana/internal/user"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
)

func main() {
	log.Println("Starting server...")

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg := config.Load()

	// Validate critical configuration
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
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

	// Initialize router
	router := mux.NewRouter()

	// Register auth feature
	authService := auth.NewService(db, cfg)
	authMiddleware := auth.NewMiddleware(cfg.JWTSecret)
	authHandler := auth.NewHandler(authService, authMiddleware)
	authHandler.RegisterRoutes(router)

	// Register user feature
	userService := user.NewService(db)
	userHandler := user.NewHandler(userService)
	userHandler.RegisterRoutes(router)

	log.Println("Server starting on :8082")
	log.Fatal(http.ListenAndServe(":8082", router))
}
