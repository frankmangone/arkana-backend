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

	router.HandleFunc("/api/posts/info", infoHandler.GetPostInfo).Methods("GET", "OPTIONS")
	router.Handle("/api/posts/like", auth.RequireAuth(http.HandlerFunc(likeHandler.ToggleLike))).Methods("POST", "OPTIONS")
	router.Handle("/api/posts/comment", auth.RequireAuth(http.HandlerFunc(commentHandler.CreateComment))).Methods("POST", "OPTIONS")
}
