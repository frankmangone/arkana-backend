package handlers

import (
	"arkana/features/posts/models"
	"arkana/features/posts/services"
	"arkana/features/wallet/middlewares"
	"arkana/shared/httputil"
	"encoding/json"
	"log"
	"net/http"
)

type LikeHandler struct {
	postService *services.PostService
}

func NewLikeHandler(ps *services.PostService) *LikeHandler {
	return &LikeHandler{postService: ps}
}

// ToggleLike handles POST /api/posts/like
func (h *LikeHandler) ToggleLike(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Like] Received like request from %s", r.RemoteAddr)

	vr, ok := middlewares.GetVerifiedRequest(r.Context())
	if !ok {
		log.Printf("[Like] Unauthorized: no verified request in context")
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	log.Printf("[Like] Verified request for wallet ID: %d, address: %s", vr.WalletID, vr.Address)

	var payload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(vr.Payload, &payload); err != nil || payload.Path == "" {
		log.Printf("[Like] Missing or invalid path in payload: %v", err)
		httputil.WriteError(w, http.StatusBadRequest, "missing path in payload")
		return
	}

	log.Printf("[Like] Processing like for path: %s", payload.Path)

	post, err := h.postService.GetOrCreateByPath(payload.Path)
	if err != nil {
		log.Printf("[Like] Failed to resolve post for path %s: %v", payload.Path, err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to resolve post")
		return
	}

	log.Printf("[Like] Post resolved: ID=%d, path=%s", post.ID, post.PathIdentifier)

	liked, likeCount, err := h.postService.ToggleLike(post.ID, vr.WalletID)
	if err != nil {
		log.Printf("[Like] Failed to toggle like for post %d, wallet %d: %v", post.ID, vr.WalletID, err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to toggle like")
		return
	}

	log.Printf("[Like] Success: post=%d, wallet=%d, liked=%v, count=%d", post.ID, vr.WalletID, liked, likeCount)

	httputil.WriteJSON(w, http.StatusOK, models.ToggleLikeResponse{
		Liked:     liked,
		LikeCount: likeCount,
	})
}
