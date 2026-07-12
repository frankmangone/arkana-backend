package handlers

import (
	"errors"
	"log"
	"net/http"

	"arkana/features/search/services"
	"arkana/shared/httputil"
)

type SearchHandler struct {
	service *services.SearchService
}

func NewSearchHandler(service *services.SearchService) *SearchHandler {
	return &SearchHandler{service: service}
}

// Search handles GET /api/search?q=<query>&lang=<lang>
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		httputil.WriteError(w, http.StatusBadRequest, "missing q parameter")
		return
	}

	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
	}

	log.Printf("[Search] query=%q lang=%q", query, lang)

	result, err := h.service.Search(lang, query)
	if err != nil {
		if errors.Is(err, services.ErrSearchUnavailable) {
			log.Printf("[Search] backend unavailable: %v", err)
			httputil.WriteError(w, http.StatusServiceUnavailable, "search is currently unavailable")
			return
		}
		log.Printf("[Search] failed: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "search failed")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}
