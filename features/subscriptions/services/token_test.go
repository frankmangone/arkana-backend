package services

import "testing"

func TestSubscriptionToken(t *testing.T) {
	t.Run("a token generated for an id/purpose verifies against the same id/purpose", func(t *testing.T) {
		token := GenerateSubscriptionToken("secret", 42, PurposeConfirm)
		if !VerifySubscriptionToken("secret", 42, PurposeConfirm, token) {
			t.Error("expected token to verify")
		}
	})

	t.Run("rejects a tampered token", func(t *testing.T) {
		token := GenerateSubscriptionToken("secret", 42, PurposeConfirm)
		if VerifySubscriptionToken("secret", 42, PurposeConfirm, token+"x") {
			t.Error("expected tampered token to be rejected")
		}
	})

	t.Run("rejects a confirm token used as an unsubscribe token", func(t *testing.T) {
		token := GenerateSubscriptionToken("secret", 42, PurposeConfirm)
		if VerifySubscriptionToken("secret", 42, PurposeUnsubscribe, token) {
			t.Error("expected cross-purpose token to be rejected")
		}
	})

	t.Run("rejects a token issued for a different subscriber id", func(t *testing.T) {
		token := GenerateSubscriptionToken("secret", 42, PurposeConfirm)
		if VerifySubscriptionToken("secret", 43, PurposeConfirm, token) {
			t.Error("expected token for a different subscriber id to be rejected")
		}
	})

	t.Run("rejects a token verified against the wrong secret", func(t *testing.T) {
		token := GenerateSubscriptionToken("secret", 42, PurposeConfirm)
		if VerifySubscriptionToken("wrong-secret", 42, PurposeConfirm, token) {
			t.Error("expected token verified with the wrong secret to be rejected")
		}
	})
}
