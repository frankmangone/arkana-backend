package handlers

import (
	"arkana/features/wallet/services"

	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, ws *services.WalletService, ts *services.TokenService) {
	loginHandler := NewLoginHandler(ws, ts)

	router.HandleFunc("/api/login", loginHandler.Login).Methods("POST")
}
