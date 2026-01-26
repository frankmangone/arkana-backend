package models

import usermodels "arkana/features/user/models"

// AuthResponse represents an authentication response with tokens
// Used by multiple handlers (signup, login, google_oauth), so kept in models
type AuthResponse struct {
	AccessToken  string            `json:"access_token"`
	RefreshToken string            `json:"refresh_token"`
	User         *usermodels.User `json:"user"`
}

// ErrorResponse represents an error response
// Used by all handlers, so kept in models as a shared utility
type ErrorResponse struct {
	Error string `json:"error"`
}
