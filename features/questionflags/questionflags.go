package questionflags

import (
	"database/sql"

	"arkana/features/auth/middlewares"
	"arkana/features/questionflags/handlers"
	"arkana/features/questionflags/services"
	"arkana/shared/adminauth"

	"github.com/gorilla/mux"
)

func Initialize(router *mux.Router, db *sql.DB, adminAuth *adminauth.AdminAuthMiddleware, auth *middlewares.AuthMiddleware) {
	service := services.NewQuestionFlagService(db)
	handlers.RegisterRoutes(router, service, auth, adminAuth)
}
