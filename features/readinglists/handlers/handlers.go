package handlers

import (
	"net/http"

	"arkana/features/readinglists/services"
	"arkana/shared/adminauth"

	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, svc *services.ReadingListService, adminAuth *adminauth.AdminAuthMiddleware) {
	adminHandler := NewAdminReadingListHandler(svc)

	router.Handle("/api/admin/reading-lists", adminAuth.RequireHMAC(http.HandlerFunc(adminHandler.Publish))).Methods("POST", "OPTIONS")
	router.Handle("/api/admin/reading-lists", adminAuth.RequireHMAC(http.HandlerFunc(adminHandler.ListAll))).Methods("GET", "OPTIONS")
}
