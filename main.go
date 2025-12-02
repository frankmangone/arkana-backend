package main

import (
	"database/sql"
	"log"
	"net/http"

	"arkana/internal/user"
	"github.com/gorilla/mux"
	"github.com/pressly/goose/v3"
)

func main() {
	log.Println("Starting server...")

	// Initialize database
	db, err := initDB("blog.db")
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

	// Register features
	userService := user.NewService(db)
	userHandler := user.NewHandler(userService)
	userHandler.RegisterRoutes(router)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func initDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	log.Println("Database connection established")
	return db, nil
}
