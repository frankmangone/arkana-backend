package router

import (
	"arkana/config"
	"arkana/features/auth"
	"arkana/features/posts"
	"database/sql"

	"github.com/gorilla/mux"
)

// Setup initializes the router and registers all routes
func Setup(db *sql.DB, cfg *config.Config) *mux.Router {
	router := mux.NewRouter()

	router.Use(CORSMiddleware(cfg.CORSAllowedOrigin))

	authMiddleware := auth.Initialize(router, db, cfg)

	posts.Initialize(router, db, authMiddleware)

	return router
}
