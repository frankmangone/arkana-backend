package handlers

import (
	"arkana/features/writers/models"
	"arkana/features/writers/services"
	"arkana/shared/httputil"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
)

type PublicWriterHandler struct {
	service *services.WriterService
}

func NewPublicWriterHandler(s *services.WriterService) *PublicWriterHandler {
	return &PublicWriterHandler{service: s}
}

// GetWriter handles GET /api/writers/{slug}.
func (h *PublicWriterHandler) GetWriter(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]

	writer, err := h.service.GetBySlug(slug)
	if err != nil {
		if errors.Is(err, services.ErrWriterNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "writer not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get writer")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, writer)
}

// ListWriters handles GET /api/writers.
func (h *PublicWriterHandler) ListWriters(w http.ResponseWriter, r *http.Request) {
	writers, err := h.service.List()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list writers")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.WriterListResponse{Data: writers})
}
