package tests

import (
	"testing"
	"time"

	"arkana/features/auth/services"
)

func TestAuthServiceGetByID(t *testing.T) {
	db := setupTestDB(t)
	svc := newAuthService(db)

	t.Run("returns nil, nil for an unknown id (not an error)", func(t *testing.T) {
		user, err := svc.GetByID(999999)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if user != nil {
			t.Errorf("user = %+v, want nil", user)
		}
	})

	t.Run("finds an existing user", func(t *testing.T) {
		created, err := svc.CreateOIDCUser("id-user@example.com", "idUser", "google", "sub-1", "")
		if err != nil {
			t.Fatal(err)
		}

		user, err := svc.GetByID(created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if user == nil {
			t.Fatal("user = nil, want a match")
		}
		if user.Email != "id-user@example.com" {
			t.Errorf("email = %q, want %q", user.Email, "id-user@example.com")
		}
	})
}

func TestAuthServiceGetUserByProviderID(t *testing.T) {
	db := setupTestDB(t)
	svc := newAuthService(db)

	t.Run("returns nil, nil when no user matches (not an error)", func(t *testing.T) {
		user, err := svc.GetUserByProviderID("google", "no-such-sub")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if user != nil {
			t.Errorf("user = %+v, want nil", user)
		}
	})

	t.Run("finds a user by provider/provider_user_id", func(t *testing.T) {
		if _, err := svc.CreateOIDCUser("provider-user@example.com", "providerUser", "google", "sub-2", ""); err != nil {
			t.Fatal(err)
		}

		user, err := svc.GetUserByProviderID("google", "sub-2")
		if err != nil {
			t.Fatal(err)
		}
		if user == nil {
			t.Fatal("user = nil, want a match")
		}
		if user.Email != "provider-user@example.com" {
			t.Errorf("email = %q, want %q", user.Email, "provider-user@example.com")
		}
	})
}

func TestAuthServiceCreateOIDCUser(t *testing.T) {
	db := setupTestDB(t)
	svc := newAuthService(db)

	user, err := svc.CreateOIDCUser("new-user@example.com", "newUser", "google", "sub-3", "https://example.com/a.png")
	if err != nil {
		t.Fatal(err)
	}

	if user.Email != "new-user@example.com" {
		t.Errorf("email = %q, want %q", user.Email, "new-user@example.com")
	}
	if user.Username == nil || *user.Username != "newUser" {
		t.Errorf("username = %v, want %q", user.Username, "newUser")
	}
	if user.AvatarURL == nil || *user.AvatarURL != "https://example.com/a.png" {
		t.Errorf("avatar_url = %v, want %q", user.AvatarURL, "https://example.com/a.png")
	}
	if !user.EmailVerified {
		t.Error("email_verified = false, want true (OIDC-created users are pre-verified)")
	}
}

func TestAuthServiceFindOrCreateGoogleUser(t *testing.T) {
	db := setupTestDB(t)
	svc := newAuthService(db)

	info := &services.GoogleUserInfo{
		Sub:       "sub-google-1",
		Email:     "google-user@example.com",
		GivenName: "Googly",
		Picture:   "https://example.com/g.png",
	}

	t.Run("creates a new user on first sign-in", func(t *testing.T) {
		user, err := svc.FindOrCreateGoogleUser(info)
		if err != nil {
			t.Fatal(err)
		}
		if user.Email != "google-user@example.com" {
			t.Errorf("email = %q, want %q", user.Email, "google-user@example.com")
		}
		if user.AuthProvider != "google" {
			t.Errorf("auth_provider = %q, want %q", user.AuthProvider, "google")
		}
	})

	t.Run("resumes (not duplicates) the same user on a second sign-in", func(t *testing.T) {
		first, err := svc.GetUserByProviderID("google", info.Sub)
		if err != nil {
			t.Fatal(err)
		}

		second, err := svc.FindOrCreateGoogleUser(info)
		if err != nil {
			t.Fatal(err)
		}
		if second.ID != first.ID {
			t.Errorf("second.ID = %d, want %d (same user, not a duplicate)", second.ID, first.ID)
		}
	})
}

func TestAuthServiceRefreshTokenLifecycle(t *testing.T) {
	db := setupTestDB(t)
	svc := newAuthService(db)

	user, err := svc.CreateOIDCUser("token-user@example.com", "tokenUser", "google", "sub-token", "")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("generates tokens and the refresh token can mint a new access token", func(t *testing.T) {
		_, refreshToken, err := svc.GenerateTokensForUser(user)
		if err != nil {
			t.Fatal(err)
		}

		accessToken, err := svc.RefreshAccessToken(refreshToken)
		if err != nil {
			t.Fatal(err)
		}
		if accessToken == "" {
			t.Error("expected a non-empty access token")
		}
	})

	t.Run("rejects an unknown refresh token", func(t *testing.T) {
		_, err := svc.RefreshAccessToken("not-a-real-token")
		if err == nil {
			t.Error("expected an error for an unknown refresh token")
		}
	})

	t.Run("rejects a revoked refresh token", func(t *testing.T) {
		_, refreshToken, err := svc.GenerateTokensForUser(user)
		if err != nil {
			t.Fatal(err)
		}

		if err := svc.RevokeRefreshToken(refreshToken); err != nil {
			t.Fatal(err)
		}

		_, err = svc.RefreshAccessToken(refreshToken)
		if err == nil {
			t.Error("expected an error for a revoked refresh token")
		}
	})

	t.Run("rejects revoking an already-revoked or unknown token", func(t *testing.T) {
		_, refreshToken, err := svc.GenerateTokensForUser(user)
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.RevokeRefreshToken(refreshToken); err != nil {
			t.Fatal(err)
		}

		if err := svc.RevokeRefreshToken(refreshToken); err == nil {
			t.Error("expected an error revoking an already-revoked token")
		}
	})

	t.Run("rejects an expired refresh token", func(t *testing.T) {
		expiredCfg := testConfig()
		expiredCfg.JWTRefreshExpiry = -time.Hour
		expiredSvc := services.NewAuthService(db, expiredCfg)

		_, refreshToken, err := expiredSvc.GenerateTokensForUser(user)
		if err != nil {
			t.Fatal(err)
		}

		_, err = expiredSvc.RefreshAccessToken(refreshToken)
		if err == nil {
			t.Error("expected an error for an expired refresh token")
		}
	})
}
