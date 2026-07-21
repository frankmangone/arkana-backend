package router

import (
	"arkana/config"
	"arkana/features/auth"
	"arkana/features/notifications"
	"arkana/features/posts"
	"arkana/features/search"
	"database/sql"

	"github.com/gorilla/mux"
)

// Setup initializes the router and registers all routes
func Setup(db *sql.DB, cfg *config.Config) *mux.Router {
	router := mux.NewRouter()

	router.Use(CORSMiddleware(cfg.CORSAllowedOrigin))

	authMiddleware := auth.Initialize(router, db, cfg)

	posts.Initialize(router, db, authMiddleware)
	search.Initialize(router, db, cfg)
	notifications.Initialize(router, db, authMiddleware)

	return router
}
