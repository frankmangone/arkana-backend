package auth

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/gorilla/mux"
)

// Handler handles auth HTTP requests
type Handler struct {
	service    *Service
	middleware *Middleware
}

// NewHandler creates a new auth handler
func NewHandler(service *Service, middleware *Middleware) *Handler {
	return &Handler{
		service:    service,
		middleware: middleware,
	}
}

// RegisterRoutes registers auth routes to the router
func (h *Handler) RegisterRoutes(router *mux.Router) {
	// Public routes
	router.HandleFunc("/api/auth/signup", h.Signup).Methods("POST")
	router.HandleFunc("/api/auth/login", h.Login).Methods("POST")
	router.HandleFunc("/api/auth/refresh", h.Refresh).Methods("POST")
	router.HandleFunc("/api/auth/logout", h.Logout).Methods("POST")

	// Protected routes
	router.Handle("/api/auth/me", h.middleware.RequireAuth(http.HandlerFunc(h.GetCurrentUser))).Methods("GET")
}

// SignupRequest represents a signup request
type SignupRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse represents an authentication response with tokens
type AuthResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         interface{} `json:"user"`
}

// Signup handles POST /api/auth/signup
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request format"})
		return
	}

	// Validate input
	if req.Email == "" || req.Username == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Email, username, and password are required"})
		return
	}

	// Validate email format
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(req.Email) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email format"})
		return
	}

	// Validate username
	if len(req.Username) < 3 || len(req.Username) > 30 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Username must be between 3 and 30 characters"})
		return
	}

	// Validate password
	if len(req.Password) < 8 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Password must be at least 8 characters"})
		return
	}

	// Register user
	user, err := h.service.Register(req.Email, req.Username, req.Password)
	if err != nil {
		// Check for duplicate email
		if err.Error() == "UNIQUE constraint failed: users.email" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Email already registered"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create user"})
		return
	}

	// Generate tokens
	accessToken, refreshToken, err := h.service.GenerateTokensForUser(user)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate tokens"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login handles POST /api/auth/login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request format"})
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Email and password are required"})
		return
	}

	// Authenticate user
	accessToken, refreshToken, user, err := h.service.Login(req.Email, req.Password)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

// RefreshRequest represents a refresh token request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// RefreshResponse represents a refresh token response
type RefreshResponse struct {
	AccessToken string `json:"access_token"`
}

// Refresh handles POST /api/auth/refresh
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request format"})
		return
	}

	if req.RefreshToken == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Refresh token is required"})
		return
	}

	// Generate new access token
	accessToken, err := h.service.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(RefreshResponse{
		AccessToken: accessToken,
	})
}

// LogoutRequest represents a logout request
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Logout handles POST /api/auth/logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request format"})
		return
	}

	if req.RefreshToken == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Refresh token is required"})
		return
	}

	// Revoke refresh token
	if err := h.service.RevokeRefreshToken(req.RefreshToken); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"})
}

// GetCurrentUser handles GET /api/auth/me
func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context (set by middleware)
	userID, ok := GetUserIDFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	// Get user from database
	user, err := h.service.GetByID(userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to retrieve user"})
		return
	}

	if user == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}
