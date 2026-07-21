package handlers

import (
	"arkana/features/auth/middlewares"
	"arkana/features/notifications/models"
	"arkana/features/notifications/services"
	"arkana/shared/httputil"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

type NotificationHandler struct {
	service *services.NotificationService
}

func NewNotificationHandler(s *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: s}
}

func RegisterRoutes(router *mux.Router, s *services.NotificationService, auth *middlewares.AuthMiddleware) {
	h := NewNotificationHandler(s)

	router.Handle("/api/notifications", auth.RequireAuth(http.HandlerFunc(h.List))).Methods("GET", "OPTIONS")
	router.Handle("/api/notifications/unread-count", auth.RequireAuth(http.HandlerFunc(h.UnreadCount))).Methods("GET", "OPTIONS")
	router.Handle("/api/notifications/{id:[0-9]+}/read", auth.RequireAuth(http.HandlerFunc(h.MarkRead))).Methods("POST", "OPTIONS")
}

// List handles GET /api/notifications
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			httputil.WriteError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		if parsed > maxLimit {
			parsed = maxLimit
		}
		limit = parsed
	}

	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			httputil.WriteError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		offset = parsed
	}

	resp, err := h.service.List(userID, limit, offset)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to fetch notifications")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

// MarkRead handles POST /api/notifications/{id}/read
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid notification id")
		return
	}

	if err := h.service.MarkRead(id, userID); err != nil {
		if errors.Is(err, services.ErrNotificationNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "notification not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to mark notification read")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.MarkReadResponse{Read: true})
}

// UnreadCount handles GET /api/notifications/unread-count
func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	count, err := h.service.UnreadCount(userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to fetch unread count")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.UnreadCountResponse{Count: count})
}
