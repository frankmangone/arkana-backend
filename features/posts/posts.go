package posts

import (
	"arkana/config"
	"arkana/features/auth/middlewares"
	notifservices "arkana/features/notifications/services"
	"arkana/features/posts/handlers"
	"arkana/features/posts/services"
	searchservices "arkana/features/search/services"
	tagservices "arkana/features/tags/services"
	"arkana/shared/adminauth"
	"database/sql"

	"github.com/gorilla/mux"
)

func Initialize(router *mux.Router, db *sql.DB, cfg *config.Config, auth *middlewares.AuthMiddleware, adminAuth *adminauth.AdminAuthMiddleware) {
	notificationService := notifservices.NewNotificationService(db)
	postService := services.NewPostService(db, notificationService)
	commentService := services.NewCommentService(db, notificationService)

	searchService := searchservices.NewSearchService(db, cfg.MeiliHost, cfg.MeiliMasterKey)
	tagService := tagservices.NewTagService(db)
	adminPostService := services.NewAdminPostService(db, postService, searchService, tagService)

	handlers.RegisterRoutes(router, postService, commentService, adminPostService, auth, adminAuth)
}
