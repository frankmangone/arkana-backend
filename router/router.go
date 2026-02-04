package router

import (
	"arkana/config"
	"arkana/features/posts"
	"arkana/features/wallet"
	"database/sql"

	"github.com/gorilla/mux"
)

// Setup initializes the router and registers all routes
func Setup(db *sql.DB, cfg *config.Config) *mux.Router {
	router := mux.NewRouter()

	// Apply CORS middleware to all routes
	router.Use(CORS)

	// Initialize wallet module (returns auth middleware for other modules)
	auth := wallet.Initialize(router, db, cfg)

	// Initialize posts module
	posts.Initialize(router, db, auth)

	return router
}
