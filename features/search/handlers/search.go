package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"arkana/features/search/services"
	"arkana/shared/httputil"
)

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 50
	maxSearchTags      = 10
)

// supportedLangs guards the values interpolated into the Meilisearch index
// UID ("posts_<lang>").
var supportedLangs = map[string]bool{
	"en": true,
	"es": true,
	"pt": true,
}

type SearchHandler struct {
	service *services.SearchService
}

func NewSearchHandler(service *services.SearchService) *SearchHandler {
	return &SearchHandler{service: service}
}

// parseTags splits a comma-separated tags parameter, dropping empty entries.
func parseTags(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		if tag := strings.TrimSpace(part); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// Search handles GET /api/search?q=<query>&lang=<lang>&tags=<a,b>&match=<all|any>&facets=tags
// Either q or tags must be present; tags alone performs a placeholder
// search (pure tag browsing).
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	tags := parseTags(r.URL.Query().Get("tags"))

	if query == "" && len(tags) == 0 {
		httputil.WriteError(w, http.StatusBadRequest, "missing q or tags parameter")
		return
	}
	if len(tags) > maxSearchTags {
		httputil.WriteError(w, http.StatusBadRequest, "too many tags (max 10)")
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

	matchAll := true
	switch r.URL.Query().Get("match") {
	case "", "all":
	case "any":
		matchAll = false
	default:
		httputil.WriteError(w, http.StatusBadRequest, "invalid match parameter (use all or any)")
		return
	}

	facets := false
	switch r.URL.Query().Get("facets") {
	case "":
	case "tags":
		facets = true
	default:
		httputil.WriteError(w, http.StatusBadRequest, "unsupported facets parameter (only tags)")
		return
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

	log.Printf("[Search] query=%q lang=%q tags=%v matchAll=%t facets=%t limit=%d",
		query, lang, tags, matchAll, facets, limit)

	result, err := h.service.Search(services.SearchParams{
		Lang:     lang,
		Query:    query,
		Tags:     tags,
		MatchAll: matchAll,
		Facets:   facets,
		Limit:    limit,
	})
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
