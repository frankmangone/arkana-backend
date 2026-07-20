package handlers

import (
	"arkana/features/auth/middlewares"
	"arkana/features/posts/services"
	"net/http"

	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, ps *services.PostService, cs *services.CommentService, auth *middlewares.AuthMiddleware) {
	likeHandler := NewLikeHandler(ps)
	commentHandler := NewCommentHandler(ps, cs)
	infoHandler := NewInfoHandler(ps)
	readHandler := NewReadHandler(ps)

	router.HandleFunc("/api/posts/{path:.*}/info", infoHandler.GetPostInfo).Methods("GET", "OPTIONS")
	router.Handle("/api/posts/{path:.*}/like", auth.RequireAuth(http.HandlerFunc(likeHandler.ToggleLike))).Methods("POST", "OPTIONS")
	router.Handle("/api/posts/reads", auth.RequireAuth(http.HandlerFunc(readHandler.GetReadStatuses))).Methods("GET", "OPTIONS")
	router.Handle("/api/posts/{path:.*}/read", auth.RequireAuth(http.HandlerFunc(readHandler.ToggleRead))).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/posts/{path:.*}/comments", commentHandler.GetComments).Methods("GET", "OPTIONS")
	router.Handle("/api/posts/{path:.*}/comments", auth.RequireAuth(http.HandlerFunc(commentHandler.CreateComment))).Methods("POST", "OPTIONS")
}
