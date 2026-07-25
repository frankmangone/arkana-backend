package handlers

import (
	"arkana/features/writers/models"
	"arkana/features/writers/services"
	"arkana/shared/httputil"
	"encoding/json"
	"net/http"
)

type AdminWriterHandler struct {
	service       *services.AdminWriterService
	writerService *services.WriterService
}

func NewAdminWriterHandler(s *services.AdminWriterService, writerSvc *services.WriterService) *AdminWriterHandler {
	return &AdminWriterHandler{service: s, writerService: writerSvc}
}

// Publish handles POST /api/admin/writers.
func (h *AdminWriterHandler) Publish(w http.ResponseWriter, r *http.Request) {
	var payload models.WriterPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil ||
		payload.Slug == "" || payload.Name == "" || payload.ImageURL == "" || payload.AvatarURL == "" {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.service.Publish(payload); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to publish writer")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.PublishWriterResponse{Published: true})
}

// ListAll handles GET /api/admin/writers, returning every writer regardless
// of visibility - for the admin-authenticated CI/build pipeline, not public
// consumption.
func (h *AdminWriterHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	writers, err := h.writerService.ListAll()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list writers")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.AdminWriterListResponse{Data: writers})
}
