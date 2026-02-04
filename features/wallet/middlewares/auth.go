package middlewares

import (
	"arkana/features/wallet/services"
	"arkana/shared/httputil"
	"context"
	"net/http"
	"strings"
)

type contextKey string

const walletClaimsKey contextKey = "walletClaims"

type AuthMiddleware struct {
	tokenService *services.TokenService
}

func NewAuthMiddleware(ts *services.TokenService) *AuthMiddleware {
	return &AuthMiddleware{tokenService: ts}
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httputil.WriteError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			httputil.WriteError(w, http.StatusUnauthorized, "invalid authorization format")
			return
		}

		claims, err := m.tokenService.ValidateToken(token)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), walletClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetWalletFromContext extracts wallet claims from the request context.
func GetWalletFromContext(ctx context.Context) (*services.WalletClaims, bool) {
	claims, ok := ctx.Value(walletClaimsKey).(*services.WalletClaims)
	return claims, ok
}
