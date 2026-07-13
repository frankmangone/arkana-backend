package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"arkana/features/search/services"
	"arkana/shared/httputil"
)

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 50
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

	limit := defaultSearchLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			httputil.WriteError(w, http.StatusBadRequest, "invalid limit parameter")
			return
		}
		limit = parsed
		if limit > maxSearchLimit {
			limit = maxSearchLimit
		}
	}

	log.Printf("[Search] query=%q lang=%q limit=%d", query, lang, limit)

	result, err := h.service.Search(lang, query, limit)
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
