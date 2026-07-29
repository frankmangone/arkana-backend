package router

import (
	"arkana/config"
	"arkana/features/auth"
	"arkana/features/notifications"
	"arkana/features/posts"
	"arkana/features/quizzes"
	"arkana/features/readinglists"
	"arkana/features/search"
	"arkana/features/subscriptions"
	"arkana/features/tags"
	"arkana/features/writers"
	"arkana/shared/adminauth"
	"database/sql"

	"github.com/gorilla/mux"
)

// Setup initializes the router and registers all routes
func Setup(db *sql.DB, cfg *config.Config) *mux.Router {
	router := mux.NewRouter()

	router.Use(CORSMiddleware(cfg.CORSAllowedOrigin))

	authMiddleware := auth.Initialize(router, db, cfg)
	adminAuthMiddleware := adminauth.NewAdminAuthMiddleware(cfg.AdminHMACSecret)

	posts.Initialize(router, db, cfg, authMiddleware, adminAuthMiddleware)
	quizzes.Initialize(router, db, adminAuthMiddleware, authMiddleware)
	readinglists.Initialize(router, db, adminAuthMiddleware)
	search.Initialize(router, db, cfg)
	notifications.Initialize(router, db, authMiddleware)
	subscriptions.Initialize(router, db, cfg, authMiddleware, adminAuthMiddleware)
	tags.Initialize(router, db, adminAuthMiddleware)
	writers.Initialize(router, db, adminAuthMiddleware)

	return router
}
