package posts

import (
	"arkana/features/auth/middlewares"
	"arkana/features/posts/handlers"
	"arkana/features/posts/services"
	"database/sql"

	"github.com/gorilla/mux"
)

func Initialize(router *mux.Router, db *sql.DB, auth *middlewares.AuthMiddleware) {
	postService := services.NewPostService(db)
	commentService := services.NewCommentService(db)

	handlers.RegisterRoutes(router, postService, commentService, auth)
}
