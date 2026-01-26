package router

import (
	"arkana/config"
	"arkana/features/auth"
	"arkana/features/user"
	"database/sql"

	"github.com/gorilla/mux"
)

// Setup initializes the router and registers all routes
func Setup(db *sql.DB, cfg *config.Config) *mux.Router {
	router := mux.NewRouter()

	// Initialize auth module
	auth.Initialize(router, db, cfg)

	// Initialize feature modules
	user.Initialize(router, db)
	// Add more features as they are created:
	// blog.Initialize(router, db)
	// comments.Initialize(router, db)

	return router
}
