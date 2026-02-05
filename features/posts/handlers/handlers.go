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
	infoHandler := NewInfoHandler(ps)

	// REST-compliant routes with path as URL parameter
	// The {path:.*} pattern captures everything including slashes
	router.HandleFunc("/api/posts/{path:.*}/info", infoHandler.GetPostInfo).Methods("GET", "OPTIONS")
	router.Handle("/api/posts/{path:.*}/like", auth.RequireAuth(http.HandlerFunc(likeHandler.ToggleLike))).Methods("POST", "OPTIONS")
	router.Handle("/api/posts/{path:.*}/comments", auth.RequireAuth(http.HandlerFunc(commentHandler.CreateComment))).Methods("POST", "OPTIONS")
}
