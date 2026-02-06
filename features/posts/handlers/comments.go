package handlers

import (
	"arkana/features/posts/services"
	"arkana/features/wallet/middlewares"
	"arkana/shared/httputil"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
)

var validate = validator.New()

type CommentHandler struct {
	postService    *services.PostService
	commentService *services.CommentService
}

func NewCommentHandler(ps *services.PostService, cs *services.CommentService) *CommentHandler {
	return &CommentHandler{postService: ps, commentService: cs}
}

// GetComments handles GET /api/posts/{path}/comments
func (h *CommentHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	path := vars["path"]
	if path == "" {
		httputil.WriteError(w, http.StatusBadRequest, "missing path in URL")
		return
	}

	post, err := h.postService.GetByPath(path)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "post not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to resolve post")
		return
	}

	comments, err := h.commentService.GetByPostID(post.ID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to fetch comments")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, comments)
}

// CreateComment handles POST /api/posts/{path}/comments
func (h *CommentHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	vr, ok := middlewares.GetVerifiedRequest(r.Context())
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

	var payload struct {
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

	post, err := h.postService.GetByPath(path)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "post not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to resolve post")
		return
	}

	comment, err := h.commentService.Create(post.ID, vr.WalletID, payload.Body, payload.ParentID)
	if err != nil {
		if errors.Is(err, services.ErrCommentTooLong) {
			httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("comment exceeds maximum length of %d characters", services.MaxCommentLength))
			return
		}
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, comment)
}
