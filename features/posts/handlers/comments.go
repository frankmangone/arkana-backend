package handlers

import (
	"arkana/features/posts/models"
	"arkana/features/posts/services"
	"arkana/features/wallet/middlewares"
	"arkana/shared/httputil"
	"encoding/json"
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

// CreateComment handles POST /api/posts/{path}/comments
func (h *CommentHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	claims, ok := middlewares.GetWalletFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	path := mux.Vars(r)["path"]

	var req models.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validate.Struct(req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	post, err := h.postService.GetOrCreateByPath(path)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to resolve post")
		return
	}

	comment, err := h.commentService.Create(post.ID, claims.WalletID, req.Body, req.ParentID)
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, comment)
}
