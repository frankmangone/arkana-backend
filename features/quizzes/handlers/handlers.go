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
	router.Handle(
		"/api/quiz-attempts/{attemptId}/next",
		auth.RequireAuth(http.HandlerFunc(sessionHandler.NextQuestion)),
	).Methods("GET", "OPTIONS")
	router.Handle(
		"/api/quiz-attempts/{attemptId}/answers",
		auth.RequireAuth(http.HandlerFunc(sessionHandler.SubmitAnswer)),
	).Methods("POST", "OPTIONS")
	router.Handle(
		"/api/quiz-attempts/{attemptId}/complete",
		auth.RequireAuth(http.HandlerFunc(sessionHandler.CompleteAttempt)),
	).Methods("POST", "OPTIONS")
}
