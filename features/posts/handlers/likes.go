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

type LikeHandler struct {
	postService *services.PostService
}

func NewLikeHandler(ps *services.PostService) *LikeHandler {
	return &LikeHandler{postService: ps}
}

// ToggleLike handles POST /api/posts/{path}/like
func (h *LikeHandler) ToggleLike(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Like] Received like request from %s", r.RemoteAddr)

	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		log.Printf("[Like] Unauthorized: no user in context")
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	log.Printf("[Like] Authenticated user ID: %d", userID)

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
		log.Printf("[Like] Failed to resolve post for path %s: %v", path, err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to resolve post")
		return
	}

	liked, likeCount, err := h.postService.ToggleLike(post.ID, userID)
	if err != nil {
		log.Printf("[Like] Failed to toggle like for post %d, user %d: %v", post.ID, userID, err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to toggle like")
		return
	}

	log.Printf("[Like] Success: post=%d, user=%d, liked=%v, count=%d", post.ID, userID, liked, likeCount)

	httputil.WriteJSON(w, http.StatusOK, models.ToggleLikeResponse{
		Liked:     liked,
		LikeCount: likeCount,
	})
}
