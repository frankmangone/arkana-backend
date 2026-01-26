package handlers

import (
	"arkana/features/auth"
	"arkana/features/user/services"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// GetUser handles GET /users/{id}
func GetUser(w http.ResponseWriter, r *http.Request, userService *services.UserService) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(auth.ErrorResponse{Error: "Invalid user ID"})
		return
	}

	user, err := userService.GetByID(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(auth.ErrorResponse{Error: "Database error"})
		return
	}

	if user == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(auth.ErrorResponse{Error: "User not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user) // TODO: Output serialization
}

// GetUserHandler returns an http.HandlerFunc that handles user retrieval
func GetUserHandler(userService *services.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		GetUser(w, r, userService)
	}
}
