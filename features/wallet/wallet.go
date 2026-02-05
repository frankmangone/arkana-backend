package wallet

import (
	"arkana/features/wallet/handlers"
	"arkana/features/wallet/middlewares"
	"arkana/features/wallet/services"
	"database/sql"

	"github.com/gorilla/mux"
)

func Initialize(router *mux.Router, db *sql.DB) *middlewares.AuthMiddleware {
	walletService := services.NewWalletService(db)

	handlers.RegisterRoutes(router, walletService)

	return middlewares.NewAuthMiddleware(walletService)
}
