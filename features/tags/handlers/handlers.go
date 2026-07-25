package handlers

import (
	"net/http"

	"arkana/features/tags/services"
	"arkana/shared/adminauth"

	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, svc *services.TagService, adminAuth *adminauth.AdminAuthMiddleware) {
	adminHandler := NewAdminTagHandler(svc)
	publicHandler := NewPublicTagHandler(svc)

	router.HandleFunc("/api/tags", publicHandler.List).Methods("GET", "OPTIONS")
	router.Handle("/api/admin/tags/sync", adminAuth.RequireHMAC(http.HandlerFunc(adminHandler.Sync))).Methods("POST", "OPTIONS")
}
