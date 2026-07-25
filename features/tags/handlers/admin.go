package handlers

import (
	"encoding/json"
	"net/http"

	"arkana/features/tags/models"
	"arkana/features/tags/services"
	"arkana/shared/httputil"
)

type AdminTagHandler struct {
	service *services.TagService
}

func NewAdminTagHandler(s *services.TagService) *AdminTagHandler {
	return &AdminTagHandler{service: s}
}

// Sync handles POST /api/admin/tags/sync - upserts the entire tags
// collection in one call. Add/update only, never deletes.
func (h *AdminTagHandler) Sync(w http.ResponseWriter, r *http.Request) {
	var req models.TagSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}
	for _, tag := range req.Tags {
		if tag.Slug == "" {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request")
			return
		}
	}

	n, err := h.service.Sync(req.Tags)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to sync tags")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.TagSyncResponse{Synced: n})
}
