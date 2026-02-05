package router

import (
	"arkana/features/posts"
	"arkana/features/wallet"
	"database/sql"

	"github.com/gorilla/mux"
)

// Setup initializes the router and registers all routes
func Setup(db *sql.DB, corsOrigin string) *mux.Router {
	router := mux.NewRouter()

	router.Use(CORSMiddleware(corsOrigin))

	// Initialize wallet module (returns auth middleware for other modules)
	auth := wallet.Initialize(router, db)

	// Initialize posts module
	posts.Initialize(router, db, auth)

	return router
}
