package writers

import (
	"arkana/features/writers/handlers"
	"arkana/features/writers/services"
	"arkana/shared/adminauth"
	"database/sql"

	"github.com/gorilla/mux"
)

func Initialize(router *mux.Router, db *sql.DB, adminAuth *adminauth.AdminAuthMiddleware) {
	adminService := services.NewAdminWriterService(db)
	writerService := services.NewWriterService(db)

	handlers.RegisterRoutes(router, adminService, writerService, adminAuth)
}
