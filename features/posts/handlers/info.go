package handlers

import (
	"arkana/features/posts/services"
	"arkana/shared/httputil"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type InfoHandler struct {
	postService *services.PostService
}

func NewInfoHandler(ps *services.PostService) *InfoHandler {
	return &InfoHandler{postService: ps}
}

// GetPostInfo handles GET /api/posts/{path}/info?user=<id>
func (h *InfoHandler) GetPostInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	path := vars["path"]

	var userID int
	if userIDStr := r.URL.Query().Get("user"); userIDStr != "" {
		if id, err := strconv.Atoi(userIDStr); err == nil {
			userID = id
		}
	}

	log.Printf("[PostInfo] Request for path=%q userID=%d", path, userID)

	if path == "" {
		httputil.WriteError(w, http.StatusBadRequest, "missing path parameter")
		return
	}

	info, err := h.postService.GetPostInfo(path, userID)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "post not found")
			return
		}
		log.Printf("[PostInfo] Failed to get post info: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get post info")
		return
	}

	log.Printf("[PostInfo] Success: path=%s, like_count=%d, liked=%v", info.Path, info.LikeCount, info.Liked)

	httputil.WriteJSON(w, http.StatusOK, info)
}
