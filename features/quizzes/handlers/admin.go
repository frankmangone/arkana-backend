package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"arkana/features/quizzes/models"
	"arkana/features/quizzes/services"
	"arkana/shared/httputil"
)

type AdminQuestionHandler struct {
	service *services.QuestionService
}

func NewAdminQuestionHandler(s *services.QuestionService) *AdminQuestionHandler {
	return &AdminQuestionHandler{service: s}
}

// Publish handles POST /api/admin/questions - upserts a batch of
// questions in one call. Add/update only, never deletes (see
// QuestionService.Publish).
func (h *AdminQuestionHandler) Publish(w http.ResponseWriter, r *http.Request) {
	var req models.QuestionPublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	for _, q := range req.Questions {
		if q.Slug == "" || q.Type == "" {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request")
			return
		}
	}

	n, err := h.service.Publish(req.Questions)
	if err != nil {
		if errors.Is(err, services.ErrUnknownPosts) || errors.Is(err, services.ErrUnknownTags) {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to publish questions")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.QuestionPublishResponse{Published: n})
}
