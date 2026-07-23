package subscriptions

import (
	"arkana/config"
	"arkana/features/auth/middlewares"
	"arkana/features/subscriptions/handlers"
	"arkana/features/subscriptions/services"
	"arkana/shared/adminauth"
	"arkana/shared/email"
	"database/sql"

	"github.com/gorilla/mux"
)

func Initialize(router *mux.Router, db *sql.DB, cfg *config.Config, auth *middlewares.AuthMiddleware, adminAuth *adminauth.AdminAuthMiddleware) {
	sender := email.NewResendSender(cfg.ResendAPIKey, cfg.ResendFromEmail)
	subscriptionService := services.NewSubscriptionService(db, sender, cfg.SubscriptionTokenSecret, cfg.FrontendURL)
	handlers.RegisterRoutes(router, subscriptionService, auth, adminAuth)
}
