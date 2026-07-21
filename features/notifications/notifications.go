package notifications

import (
	"arkana/features/auth/middlewares"
	"arkana/features/notifications/handlers"
	"arkana/features/notifications/services"
	"database/sql"

	"github.com/gorilla/mux"
)

func Initialize(router *mux.Router, db *sql.DB, auth *middlewares.AuthMiddleware) {
	notificationService := services.NewNotificationService(db)
	handlers.RegisterRoutes(router, notificationService, auth)
}
