package handlers

import (
	"errors"
	"net/http"

	"arkana/features/auth/middlewares"
	"arkana/features/quizzes/models"
	"arkana/features/quizzes/services"
	"arkana/shared/httputil"

	"github.com/gorilla/mux"
)

type SessionHandler struct {
	service *services.QuizSessionService
}

func NewSessionHandler(s *services.QuizSessionService) *SessionHandler {
	return &SessionHandler{service: s}
}

// StartAttempt handles POST /api/reading-lists/{listSlug}/modules/{moduleSlug}/quiz/attempts.
func (h *SessionHandler) StartAttempt(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	vars := mux.Vars(r)
	attemptUUID, total, err := h.service.Start(userID, vars["listSlug"], vars["moduleSlug"])
	if err != nil {
		if errors.Is(err, services.ErrModuleNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "module not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to start attempt")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.StartAttemptResponse{AttemptID: attemptUUID, TotalQuestions: total})
}
