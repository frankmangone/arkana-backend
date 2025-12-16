package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const UserIDContextKey contextKey = "user_id"
const EmailContextKey contextKey = "email"

// Middleware handles authentication middleware
type Middleware struct {
	jwtSecret string
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(jwtSecret string) *Middleware {
	return &Middleware{jwtSecret: jwtSecret}
}

// RequireAuth is middleware that requires valid authentication
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing authorization token"})
			return
		}

		claims, err := ValidateAccessToken(token, m.jwtSecret)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or expired token"})
			return
		}

		// Attach user info to context
		ctx := context.WithValue(r.Context(), UserIDContextKey, claims.UserID)
		ctx = context.WithValue(ctx, EmailContextKey, claims.Email)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth is middleware that attaches user info if a valid token is present
func (m *Middleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token != "" {
			claims, err := ValidateAccessToken(token, m.jwtSecret)
			if err == nil {
				// Attach user info to context if valid
				ctx := context.WithValue(r.Context(), UserIDContextKey, claims.UserID)
				ctx = context.WithValue(ctx, EmailContextKey, claims.Email)
				r = r.WithContext(ctx)
			}
		}

		next.ServeHTTP(w, r)
	})
}

// extractToken extracts the JWT token from the Authorization header
func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// Expected format: "Bearer <token>"
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}

// GetUserIDFromContext retrieves the user ID from the request context
func GetUserIDFromContext(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(UserIDContextKey).(int)
	return userID, ok
}

// GetEmailFromContext retrieves the email from the request context
func GetEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(EmailContextKey).(string)
	return email, ok
}
