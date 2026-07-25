package handlers

import (
	"net/http"

	"arkana/features/tags/models"
	"arkana/features/tags/services"
	"arkana/shared/httputil"
)

type PublicTagHandler struct {
	service *services.TagService
}

func NewPublicTagHandler(s *services.TagService) *PublicTagHandler {
	return &PublicTagHandler{service: s}
}

// List handles GET /api/tags. Public, unauthenticated - tags have no
// hidden/visibility concept, so there's no reason to gate this behind
// HMAC the way admin-only listings are for other features.
func (h *PublicTagHandler) List(w http.ResponseWriter, r *http.Request) {
	tags, err := h.service.List()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list tags")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.TagListResponse{Data: tags})
}
