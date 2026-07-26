package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"arkana/features/readinglists/models"
	"arkana/features/readinglists/services"
	"arkana/shared/httputil"
)

type AdminReadingListHandler struct {
	service *services.ReadingListService
}

func NewAdminReadingListHandler(s *services.ReadingListService) *AdminReadingListHandler {
	return &AdminReadingListHandler{service: s}
}

// Publish handles POST /api/admin/reading-lists - full-replaces one
// reading list's structure by slug.
func (h *AdminReadingListHandler) Publish(w http.ResponseWriter, r *http.Request) {
	var req models.ReadingListPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Slug == "" || len(req.Translations) == 0 {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.service.Publish(req); err != nil {
		if errors.Is(err, services.ErrUnknownPosts) {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to publish reading list")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.PublishReadingListResponse{Published: true})
}

// ListAll handles GET /api/admin/reading-lists, returning every reading
// list fully nested - for the admin-authenticated CI content pull, not
// public consumption.
func (h *AdminReadingListHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	lists, err := h.service.ListAll()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list reading lists")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.AdminReadingListListResponse{Data: lists})
}
