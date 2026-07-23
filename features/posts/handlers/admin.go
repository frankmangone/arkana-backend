package handlers

import (
	"arkana/features/posts/models"
	"arkana/features/posts/services"
	"arkana/shared/httputil"
	"encoding/json"
	"net/http"
)

type AdminPostHandler struct {
	service *services.AdminPostService
}

func NewAdminPostHandler(s *services.AdminPostService) *AdminPostHandler {
	return &AdminPostHandler{service: s}
}

// Publish handles POST /api/admin/posts.
func (h *AdminPostHandler) Publish(w http.ResponseWriter, r *http.Request) {
	var req models.PublishPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" || req.Lang == "" || req.RawContent == "" {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	err := h.service.Publish(services.PublishInput{
		Path:       req.Path,
		Lang:       req.Lang,
		RawContent: req.RawContent,
	})
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to publish post")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.PublishPostResponse{Published: true})
}
