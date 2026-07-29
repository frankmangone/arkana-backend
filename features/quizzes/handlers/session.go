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

// supportedLangs mirrors features/search/handlers/search.go's set of the
// same name - kept as its own copy since that one is unexported.
var supportedLangs = map[string]bool{"en": true, "es": true, "pt": true}

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

// NextQuestion handles GET /api/quiz-attempts/{attemptId}/next.
func (h *SessionHandler) NextQuestion(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
	}
	if !supportedLangs[lang] {
		httputil.WriteError(w, http.StatusBadRequest, "unsupported lang parameter")
		return
	}

	vars := mux.Vars(r)
	q, position, total, done, err := h.service.Next(userID, vars["attemptId"], lang)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAttemptNotFound), errors.Is(err, services.ErrAttemptForbidden):
			// Same 404 for both - a non-owner must never learn that an
			// attempt uuid they don't own actually exists.
			httputil.WriteError(w, http.StatusNotFound, "attempt not found")
		case errors.Is(err, services.ErrAttemptCompleted):
			httputil.WriteError(w, http.StatusConflict, "attempt already completed")
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "failed to load next question")
		}
		return
	}

	resp := models.NextQuestionResponse{Position: position, TotalQuestions: total, Done: done}
	if q != nil {
		resp.Question = &models.QuestionDTO{UUID: q.UUID, Type: q.Type, Difficulty: q.Difficulty, Prompt: q.Prompt, Content: q.Content}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}
