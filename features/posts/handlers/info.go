package handlers

import (
	"arkana/features/posts/services"
	"arkana/shared/httputil"
	"log"
	"net/http"
)

type InfoHandler struct {
	postService *services.PostService
}

func NewInfoHandler(ps *services.PostService) *InfoHandler {
	return &InfoHandler{postService: ps}
}

// GetPostInfo handles GET /api/posts/info?path=xxx&wallet=xxx
func (h *InfoHandler) GetPostInfo(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	wallet := r.URL.Query().Get("wallet")

	log.Printf("[PostInfo] Request for path=%q wallet=%q", path, wallet)

	if path == "" {
		log.Printf("[PostInfo] Missing path parameter")
		httputil.WriteError(w, http.StatusBadRequest, "missing path parameter")
		return
	}

	info, err := h.postService.GetPostInfo(path, wallet)
	if err != nil {
		log.Printf("[PostInfo] Failed to get post info: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get post info")
		return
	}

	log.Printf("[PostInfo] Success: path=%s, like_count=%d, liked=%v", info.Path, info.LikeCount, info.Liked)

	httputil.WriteJSON(w, http.StatusOK, info)
}
