package handlers

import (
	"arkana/features/auth/middlewares"
	"arkana/features/posts/models"
	"arkana/features/posts/services"
	"arkana/shared/httputil"
	"errors"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type ReadHandler struct {
	postService *services.PostService
}

func NewReadHandler(ps *services.PostService) *ReadHandler {
	return &ReadHandler{postService: ps}
}

// ToggleRead handles POST /api/posts/{path}/read
func (h *ReadHandler) ToggleRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	vars := mux.Vars(r)
	path := vars["path"]
	if path == "" {
		httputil.WriteError(w, http.StatusBadRequest, "missing path in URL")
		return
	}

	post, err := h.postService.GetOrCreateByPath(path)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "post not found")
			return
		}
		log.Printf("[Read] Failed to resolve post for path %s: %v", path, err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to resolve post")
		return
	}

	read, err := h.postService.ToggleRead(post.ID, userID)
	if err != nil {
		log.Printf("[Read] Failed to toggle read for post %d, user %d: %v", post.ID, userID, err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to toggle read")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.ToggleReadResponse{Read: read})
}
