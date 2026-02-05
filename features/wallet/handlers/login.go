package handlers

import (
	"arkana/features/wallet/models"
	"arkana/features/wallet/services"
	"arkana/shared/httputil"
	"io"
	"log"
	"net/http"
)

type LoginHandler struct {
	walletService *services.WalletService
}

func NewLoginHandler(ws *services.WalletService) *LoginHandler {
	return &LoginHandler{walletService: ws}
}

func (h *LoginHandler) Login(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Login] Received login request from %s", r.RemoteAddr)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[Login] Failed to read request body: %v", err)
		httputil.WriteError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	log.Printf("[Login] Request body length: %d bytes", len(body))

	envelope, err := services.ParseCompactJWS(string(body))
	if err != nil {
		log.Printf("[Login] Failed to parse JWS: %v", err)
		httputil.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("[Login] JWS parsed successfully")

	verified, err := services.VerifyJWS(envelope)
	if err != nil {
		log.Printf("[Login] JWS verification failed: %v", err)
		httputil.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	log.Printf("[Login] JWS verified for address: %s (system: %s)", verified.Address, verified.Header.System)

	wallet, err := h.walletService.GetOrCreate(verified.Address, verified.Header.System)
	if err != nil {
		log.Printf("[Login] Failed to get/create wallet: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "failed to process wallet")
		return
	}

	log.Printf("[Login] Login successful for wallet ID: %d, address: %s", wallet.ID, wallet.Address)

	httputil.WriteJSON(w, http.StatusOK, models.LoginResponse{
		Wallet: *wallet,
	})
}
