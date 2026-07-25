package handlers

import (
	"arkana/features/writers/services"
	"arkana/shared/adminauth"
	"net/http"

	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, adminSvc *services.AdminWriterService, writerSvc *services.WriterService, adminAuth *adminauth.AdminAuthMiddleware) {
	adminHandler := NewAdminWriterHandler(adminSvc)
	publicHandler := NewPublicWriterHandler(writerSvc)

	router.HandleFunc("/api/writers", publicHandler.ListWriters).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/writers/{slug}", publicHandler.GetWriter).Methods("GET", "OPTIONS")
	router.Handle("/api/admin/writers", adminAuth.RequireHMAC(http.HandlerFunc(adminHandler.Publish))).Methods("POST", "OPTIONS")
}
