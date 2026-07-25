package handlers

import (
	"arkana/features/posts/models"
	"arkana/features/posts/services"
	"arkana/shared/httputil"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

const (
	defaultContentLimit = 20
	maxContentLimit     = 100
)

type AdminPostHandler struct {
	service *services.AdminPostService
	posts   *services.PostService
}

func NewAdminPostHandler(s *services.AdminPostService, posts *services.PostService) *AdminPostHandler {
	return &AdminPostHandler{service: s, posts: posts}
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
		if errors.Is(err, services.ErrUnknownTags) {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to publish post")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.PublishPostResponse{Published: true})
}

// ListContent handles GET /api/admin/posts, returning a page of visible
// post_contents rows for the CI content pull - not for public consumption.
func (h *AdminPostHandler) ListContent(w http.ResponseWriter, r *http.Request) {
	limit := defaultContentLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			httputil.WriteError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		if parsed > maxContentLimit {
			parsed = maxContentLimit
		}
		limit = parsed
	}

	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			httputil.WriteError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		offset = parsed
	}

	items, total, err := h.posts.ListVisibleContentPage(limit, offset)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to list post content")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.AdminPostContentListResponse{Data: items, Total: total})
}
