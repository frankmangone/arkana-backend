package wallet

import (
	"arkana/config"
	"arkana/features/wallet/handlers"
	"arkana/features/wallet/middlewares"
	"arkana/features/wallet/services"
	"database/sql"

	"github.com/gorilla/mux"
)

func Initialize(router *mux.Router, db *sql.DB, cfg *config.Config) *middlewares.AuthMiddleware {
	walletService := services.NewWalletService(db)
	tokenService := services.NewTokenService(cfg.SigningSecret, cfg.TokenExpiry)

	handlers.RegisterRoutes(router, walletService, tokenService)

	return middlewares.NewAuthMiddleware(tokenService)
}
