package handlers

import (
	"arkana/features/wallet/services"

	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, ws *services.WalletService) {
	loginHandler := NewLoginHandler(ws)

	router.HandleFunc("/api/login", loginHandler.Login).Methods("POST")
}
