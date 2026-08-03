package handlers

import (
	"arkana/features/auth/middlewares"
	"arkana/features/auth/services"
	"arkana/shared/httputil"
	"net/http"
)

// MeHandler handles GET /api/auth/me
func MeHandler(authService *services.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middlewares.GetUserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		user, err := authService.GetByID(userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "Failed to retrieve user")
			return
		}

		if user == nil {
			httputil.WriteError(w, http.StatusNotFound, "User not found")
			return
		}

		httputil.WriteJSON(w, http.StatusOK, user)
	}
}
