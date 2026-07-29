package handlers

import (
	"net/http"

	"arkana/features/auth/middlewares"
	"arkana/features/quizzes/services"
	"arkana/shared/adminauth"

	"github.com/gorilla/mux"
)

// sessions and auth are accepted now (rather than added to this
// signature later) so RegisterRoutes's signature doesn't change again
// once Tasks 4-7 register session routes against them - unused function
// parameters are not a compile error in Go, so no placeholder is needed
// for them here.
func RegisterRoutes(
	router *mux.Router,
	questions *services.QuestionService,
	sessions *services.QuizSessionService,
	auth *middlewares.AuthMiddleware,
	adminAuth *adminauth.AdminAuthMiddleware,
) {
	adminHandler := NewAdminQuestionHandler(questions)

	router.Handle("/api/admin/questions", adminAuth.RequireHMAC(http.HandlerFunc(adminHandler.Publish))).Methods("POST", "OPTIONS")
}
