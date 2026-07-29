package handlers

import (
	"encoding/json"
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

// SubmitAnswer handles POST /api/quiz-attempts/{attemptId}/answers. Body
// must contain exactly one of response or skipped.
func (h *SessionHandler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
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

	var req models.AnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	hasResponse := len(req.Response) > 0
	if req.QuestionID == "" || hasResponse == req.Skipped {
		httputil.WriteError(w, http.StatusBadRequest, "exactly one of response or skipped must be present")
		return
	}

	vars := mux.Vars(r)
	result, err := h.service.Answer(userID, vars["attemptId"], req.QuestionID, req.Response, req.Skipped, lang)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAttemptNotFound), errors.Is(err, services.ErrAttemptForbidden):
			httputil.WriteError(w, http.StatusNotFound, "attempt not found")
		case errors.Is(err, services.ErrAttemptCompleted):
			httputil.WriteError(w, http.StatusConflict, "attempt already completed")
		case errors.Is(err, services.ErrWrongQuestion):
			httputil.WriteError(w, http.StatusConflict, err.Error())
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "failed to submit answer")
		}
		return
	}

	resp := models.AnswerResponse{Correct: result.Correct, Skipped: result.Skipped, AttemptDone: result.AttemptDone}
	if !result.Correct {
		resp.CorrectReveal = result.CorrectReveal
		resp.Explanation = result.Explanation
		if len(result.PostPaths) > 0 {
			resp.Reinforcement = &models.ReinforcementDTO{PostPaths: result.PostPaths}
		}
	}
	httputil.WriteJSON(w, http.StatusOK, resp)
}

// CompleteAttempt handles POST /api/quiz-attempts/{attemptId}/complete.
func (h *SessionHandler) CompleteAttempt(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	vars := mux.Vars(r)
	result, err := h.service.Complete(userID, vars["attemptId"])
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAttemptNotFound), errors.Is(err, services.ErrAttemptForbidden):
			httputil.WriteError(w, http.StatusNotFound, "attempt not found")
		case errors.Is(err, services.ErrAttemptCompleted):
			httputil.WriteError(w, http.StatusConflict, "attempt already completed")
		case errors.Is(err, services.ErrAttemptIncomplete):
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			httputil.WriteError(w, http.StatusInternalServerError, "failed to complete attempt")
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.CompleteAttemptResponse{Score: result.Score, Passed: result.Passed})
}
