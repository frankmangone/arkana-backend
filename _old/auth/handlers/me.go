package handlers

import (
	"arkana/features/auth/middlewares"
	"arkana/features/auth/models"
	"arkana/features/auth/services"
	"encoding/json"
	"net/http"
)

// GetCurrentUser handles GET /auth/me
func GetCurrentUser(w http.ResponseWriter, r *http.Request, authService *services.AuthService) {
	// Get user ID from context (set by middleware)
	userID, ok := middlewares.GetUserIDFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Unauthorized"})
		return
	}

	// Get user from database
	user, err := authService.GetByID(userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Failed to retrieve user"})
		return
	}

	if user == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "User not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}

// GetCurrentUserHandler returns an http.HandlerFunc that handles getting current user
func GetCurrentUserHandler(authService *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		GetCurrentUser(w, r, authService)
	}
}
