package handlers

import (
	"arkana/features/wallet/models"
	"arkana/features/wallet/services"
	"arkana/shared/httputil"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

const maxMessageAge = 5 * time.Minute

type LoginHandler struct {
	walletService *services.WalletService
	tokenService  *services.TokenService
}

func NewLoginHandler(ws *services.WalletService, ts *services.TokenService) *LoginHandler {
	return &LoginHandler{
		walletService: ws,
		tokenService:  ts,
	}
}

func (h *LoginHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validate.Struct(req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	// Parse and validate timestamp from message
	ts, err := parseMessageTimestamp(req.Message)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid message format")
		return
	}

	age := time.Since(time.Unix(ts, 0))
	if age > maxMessageAge || age < -maxMessageAge {
		httputil.WriteError(w, http.StatusUnauthorized, "message expired")
		return
	}

	// Verify signature
	if err := services.VerifySignature(req.System, req.Address, req.Message, req.Signature); err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	// Upsert wallet
	wallet, err := h.walletService.GetOrCreate(req.Address, req.System)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to process wallet")
		return
	}

	// Generate token
	token, err := h.tokenService.GenerateToken(wallet.ID, wallet.Address, wallet.System)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, models.LoginResponse{
		Token:  token,
		Wallet: *wallet,
	})
}

// parseMessageTimestamp extracts the unix timestamp from "Arkana login: <timestamp>"
func parseMessageTimestamp(message string) (int64, error) {
	const prefix = "Arkana login: "
	if !strings.HasPrefix(message, prefix) {
		return 0, fmt.Errorf("invalid message prefix")
	}

	tsStr := strings.TrimPrefix(message, prefix)
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp: %w", err)
	}

	return ts, nil
}
