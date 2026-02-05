package middlewares

import (
	"arkana/features/wallet/services"
	"arkana/shared/httputil"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type contextKey string

const verifiedRequestKey contextKey = "verifiedRequest"

// VerifiedRequest holds the result of JWS verification, attached to request context.
type VerifiedRequest struct {
	WalletID int
	Address  string
	System   string
	Payload  json.RawMessage
}

type AuthMiddleware struct {
	walletService *services.WalletService
}

func NewAuthMiddleware(ws *services.WalletService) *AuthMiddleware {
	return &AuthMiddleware{walletService: ws}
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "failed to read request body")
			return
		}
		defer r.Body.Close()

		envelope, err := services.ParseCompactJWS(string(body))
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		verified, err := services.VerifyJWS(envelope)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, err.Error())
			return
		}

		wallet, err := m.walletService.GetByAddress(verified.Address)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, "wallet not found")
			return
		}

		ctx := context.WithValue(r.Context(), verifiedRequestKey, &VerifiedRequest{
			WalletID: wallet.ID,
			Address:  wallet.Address,
			System:   verified.Header.System,
			Payload:  verified.Payload,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetVerifiedRequest extracts the verified JWS data from the request context.
func GetVerifiedRequest(ctx context.Context) (*VerifiedRequest, bool) {
	vr, ok := ctx.Value(verifiedRequestKey).(*VerifiedRequest)
	return vr, ok
}
