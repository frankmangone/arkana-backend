package handlers

import (
	"arkana/features/auth/middlewares"
	"arkana/features/subscriptions/models"
	"arkana/features/subscriptions/services"
	"arkana/shared/adminauth"
	"arkana/shared/httputil"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
)

type SubscriptionHandler struct {
	service *services.SubscriptionService
}

func NewSubscriptionHandler(s *services.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{service: s}
}

func RegisterRoutes(router *mux.Router, s *services.SubscriptionService, auth *middlewares.AuthMiddleware, adminAuth *adminauth.AdminAuthMiddleware) {
	h := NewSubscriptionHandler(s)

	router.Handle("/api/subscribe", http.HandlerFunc(h.Subscribe)).Methods("POST", "OPTIONS")
	router.Handle("/api/subscribe/confirm", http.HandlerFunc(h.Confirm)).Methods("POST", "OPTIONS")
	router.Handle("/api/subscriptions", auth.RequireAuth(http.HandlerFunc(h.AuthenticatedSubscribe))).Methods("POST", "OPTIONS")
	router.Handle("/api/subscriptions", auth.RequireAuth(http.HandlerFunc(h.GetStatus))).Methods("GET", "OPTIONS")
	router.Handle("/api/subscriptions", auth.RequireAuth(http.HandlerFunc(h.AuthenticatedUnsubscribe))).Methods("DELETE", "OPTIONS")
	router.Handle("/api/subscriptions/unsubscribe", http.HandlerFunc(h.Unsubscribe)).Methods("POST", "OPTIONS")
	router.Handle("/api/admin/subscriptions/broadcast", adminAuth.RequireHMAC(http.HandlerFunc(h.Broadcast))).Methods("POST", "OPTIONS")
}

// Subscribe handles POST /api/subscribe (guest signup).
func (h *SubscriptionHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req models.SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.service.Subscribe(r.Context(), req.Email); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to process subscription")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.SubscribeResponse{Status: "pending"})
}

// Confirm handles POST /api/subscribe/confirm.
func (h *SubscriptionHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	var req models.ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.service.ConfirmSubscription(r.Context(), req.SubscriberID, req.Token); err != nil {
		if errors.Is(err, services.ErrInvalidToken) {
			httputil.WriteError(w, http.StatusBadRequest, "invalid or expired link")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to confirm subscription")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.ConfirmResponse{Confirmed: true})
}

// AuthenticatedSubscribe handles POST /api/subscriptions.
func (h *SubscriptionHandler) AuthenticatedSubscribe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	userEmail, ok := middlewares.GetEmailFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.AuthenticatedSubscribe(r.Context(), userID, userEmail); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to subscribe")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.SubscriptionStatusResponse{Subscribed: true})
}

// GetStatus handles GET /api/subscriptions.
func (h *SubscriptionHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	subscribed, err := h.service.GetStatus(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get subscription status")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.SubscriptionStatusResponse{Subscribed: subscribed})
}

// AuthenticatedUnsubscribe handles DELETE /api/subscriptions.
func (h *SubscriptionHandler) AuthenticatedUnsubscribe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.AuthenticatedUnsubscribe(r.Context(), userID); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to unsubscribe")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.SubscriptionStatusResponse{Subscribed: false})
}

// Unsubscribe handles POST /api/subscriptions/unsubscribe (public, one-click).
func (h *SubscriptionHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	var req models.ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.service.UnsubscribeByToken(r.Context(), req.SubscriberID, req.Token); err != nil {
		if errors.Is(err, services.ErrInvalidToken) {
			httputil.WriteError(w, http.StatusBadRequest, "invalid or expired link")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to unsubscribe")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.UnsubscribeResponse{Unsubscribed: true})
}

// Broadcast handles POST /api/admin/subscriptions/broadcast.
func (h *SubscriptionHandler) Broadcast(w http.ResponseWriter, r *http.Request) {
	var req models.BroadcastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	sent, failed, err := h.service.Broadcast(r.Context(), req.PostID)
	if err != nil {
		if errors.Is(err, services.ErrPostNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "post not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "failed to send broadcast")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.BroadcastResponse{Sent: sent, Failed: failed})
}
