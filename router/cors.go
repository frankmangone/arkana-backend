package router

import (
	"net/http"
	"os"
	"strings"
)

// CORS middleware handles Cross-Origin Resource Sharing
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get allowed origin from environment or use request origin
		allowedOrigin := os.Getenv("CORS_ALLOWED_ORIGIN")
		origin := r.Header.Get("Origin")

		if allowedOrigin == "" {
			// For development, allow common localhost origins
			if origin == "http://localhost:3333" || origin == "http://localhost:3000" {
				allowedOrigin = origin
			} else if origin != "" {
				// Allow the requesting origin if it's a localhost variant
				if strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "https://localhost:") {
					allowedOrigin = origin
				} else {
					// For other origins, use wildcard (less secure but works for dev)
					allowedOrigin = "*"
				}
			} else {
				// If no origin header (e.g., same-origin request), allow all for dev
				allowedOrigin = "*"
			}
		}

		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "3600") // Cache preflight for 1 hour

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}
