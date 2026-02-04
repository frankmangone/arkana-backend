package handlers

import (
	"arkana/features/posts/services"
	"arkana/features/wallet/middlewares"
	"net/http"

	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, ps *services.PostService, cs *services.CommentService, auth *middlewares.AuthMiddleware) {
	likeHandler := NewLikeHandler(ps)
	commentHandler := NewCommentHandler(ps, cs)

	router.Handle("/api/posts/{path}/like", auth.RequireAuth(http.HandlerFunc(likeHandler.ToggleLike))).Methods("POST")
	router.Handle("/api/posts/{path}/comments", auth.RequireAuth(http.HandlerFunc(commentHandler.CreateComment))).Methods("POST")
}
