package posts

import (
	"arkana/features/auth/middlewares"
	notifservices "arkana/features/notifications/services"
	"arkana/features/posts/handlers"
	"arkana/features/posts/services"
	"database/sql"

	"github.com/gorilla/mux"
)

func Initialize(router *mux.Router, db *sql.DB, auth *middlewares.AuthMiddleware) {
	notificationService := notifservices.NewNotificationService(db)
	postService := services.NewPostService(db, notificationService)
	commentService := services.NewCommentService(db, notificationService)

	handlers.RegisterRoutes(router, postService, commentService, auth)
}
