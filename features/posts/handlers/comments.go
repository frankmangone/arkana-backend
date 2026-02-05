package handlers

import (
	"arkana/features/posts/services"
	"arkana/features/wallet/middlewares"
	"arkana/shared/httputil"
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type CommentHandler struct {
	postService    *services.PostService
	commentService *services.CommentService
}

func NewCommentHandler(ps *services.PostService, cs *services.CommentService) *CommentHandler {
	return &CommentHandler{postService: ps, commentService: cs}
}

// CreateComment handles POST /api/posts/comment
func (h *CommentHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	vr, ok := middlewares.GetVerifiedRequest(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var payload struct {
		Path     string `json:"path" validate:"required"`
		Body     string `json:"body" validate:"required"`
		ParentID *int   `json:"parent_id,omitempty"`
	}
	if err := json.Unmarshal(vr.Payload, &payload); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if err := validate.Struct(payload); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid payload: "+err.Error())
		return
	}

	post, err := h.postService.GetOrCreateByPath(payload.Path)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to resolve post")
		return
	}

	comment, err := h.commentService.Create(post.ID, vr.WalletID, payload.Body, payload.ParentID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, comment)
}
