package posts

import (
	"arkana/features/posts/handlers"
	"arkana/features/posts/services"
	"arkana/features/wallet/middlewares"
	"database/sql"

	"github.com/gorilla/mux"
)

func Initialize(router *mux.Router, db *sql.DB, auth *middlewares.AuthMiddleware) {
	postService := services.NewPostService(db)
	commentService := services.NewCommentService(db)

	handlers.RegisterRoutes(router, postService, commentService, auth)
}
