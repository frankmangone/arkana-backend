package handlers

import (
	"arkana/features/writers/services"
	"arkana/shared/adminauth"
	"net/http"

	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, adminSvc *services.AdminWriterService, adminAuth *adminauth.AdminAuthMiddleware) {
	adminHandler := NewAdminWriterHandler(adminSvc)

	router.Handle("/api/admin/writers", adminAuth.RequireHMAC(http.HandlerFunc(adminHandler.Publish))).Methods("POST", "OPTIONS")
}
