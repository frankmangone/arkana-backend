package router

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// CORSMiddleware returns a middleware that sets CORS headers using the configured origin.
func CORSMiddleware(allowedOrigin string) mux.MiddlewareFunc {
	if allowedOrigin == "" {
		allowedOrigin = "*"
	}

	log.Printf("[CORS] Middleware initialized with allowed origin: %q", allowedOrigin)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			log.Printf("[CORS] %s %s | Origin: %q | Allowed: %q", r.Method, r.URL.Path, origin, allowedOrigin)

			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "3600")

			if r.Method == "OPTIONS" {
				log.Printf("[CORS] Preflight request handled for %s", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
