package handlers

import (
	"net/http"

	"arkana/features/auth/middlewares"
	"arkana/features/quizzes/services"
	"arkana/shared/adminauth"

	"github.com/gorilla/mux"
)

func RegisterRoutes(
	router *mux.Router,
	questions *services.QuestionService,
	sessions *services.QuizSessionService,
	auth *middlewares.AuthMiddleware,
	adminAuth *adminauth.AdminAuthMiddleware,
) {
	adminHandler := NewAdminQuestionHandler(questions)
	sessionHandler := NewSessionHandler(sessions)

	router.Handle("/api/admin/questions", adminAuth.RequireHMAC(http.HandlerFunc(adminHandler.Publish))).Methods("POST", "OPTIONS")
	router.Handle(
		"/api/reading-lists/{listSlug}/modules/{moduleSlug}/quiz/attempts",
		auth.RequireAuth(http.HandlerFunc(sessionHandler.StartAttempt)),
	).Methods("POST", "OPTIONS")
}
