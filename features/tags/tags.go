package tags

import (
	"arkana/features/tags/handlers"
	"arkana/features/tags/services"
	"arkana/shared/adminauth"
	"database/sql"

	"github.com/gorilla/mux"
)

func Initialize(router *mux.Router, db *sql.DB, adminAuth *adminauth.AdminAuthMiddleware) {
	svc := services.NewTagService(db)
	handlers.RegisterRoutes(router, svc, adminAuth)
}
