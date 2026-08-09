package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"arkana/features/auth/middlewares"
	"arkana/features/questionflags/models"
	"arkana/features/questionflags/services"
	"arkana/shared/adminauth"
	"arkana/shared/httputil"

	"github.com/gorilla/mux"
)

type QuestionFlagHandler struct {
	service *services.QuestionFlagService
}

func NewQuestionFlagHandler(s *services.QuestionFlagService) *QuestionFlagHandler {
	return &QuestionFlagHandler{service: s}
}

func RegisterRoutes(router *mux.Router, s *services.QuestionFlagService, auth *middlewares.AuthMiddleware, adminAuth *adminauth.AdminAuthMiddleware) {
	h := NewQuestionFlagHandler(s)

	router.Handle("/api/questions/{uuid}/flags", auth.RequireAuth(http.HandlerFunc(h.Create))).Methods("POST", "OPTIONS")
	router.Handle("/api/admin/question-flags", adminAuth.RequireHMAC(http.HandlerFunc(h.List))).Methods("GET", "OPTIONS")
	router.Handle("/api/admin/question-flags", adminAuth.RequireHMAC(http.HandlerFunc(h.Delete))).Methods("DELETE", "OPTIONS")
}

// Create handles POST /api/questions/{uuid}/flags
func (h *QuestionFlagHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	vars := mux.Vars(r)
	questionUUID := vars["uuid"]

	var payload struct {
		Reason string `json:"reason" validate:"required"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := httputil.ValidateRequest(payload); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid payload: "+err.Error())
		return
	}

	flag, err := h.service.Create(questionUUID, userID, payload.Reason)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "question not found")
			return
		}
		if errors.Is(err, services.ErrReasonTooLong) {
			httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("reason exceeds maximum length of %d characters", services.MaxFlagReasonLength))
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to save flag")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, flag)
}

// List handles GET /api/admin/question-flags
func (h *QuestionFlagHandler) List(w http.ResponseWriter, r *http.Request) {
	flags, err := h.service.List()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to fetch flags")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.ListFlagsResponse{Flags: flags})
}

// Delete handles DELETE /api/admin/question-flags, and
// DELETE /api/admin/question-flags?id={id} for a single flag.
func (h *QuestionFlagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	var deleted int64
	var err error

	if raw := r.URL.Query().Get("id"); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid id")
			return
		}
		deleted, err = h.service.Delete(id)
	} else {
		deleted, err = h.service.DeleteAll()
	}
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to delete flags")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.DeleteFlagsResponse{Deleted: deleted})
}
