package handlers

import (
	"arkana/features/posts/models"
	"arkana/features/posts/services"
	"arkana/features/wallet/middlewares"
	"arkana/shared/httputil"
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
	claims, ok := middlewares.GetWalletFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	path := mux.Vars(r)["path"]

	post, err := h.postService.GetOrCreateByPath(path)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to resolve post")
		return
	}

	liked, likeCount, err := h.postService.ToggleLike(post.ID, claims.WalletID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to toggle like")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.ToggleLikeResponse{
		Liked:     liked,
		LikeCount: likeCount,
	})
}
