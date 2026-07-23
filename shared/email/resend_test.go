package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResendSenderSend(t *testing.T) {
	t.Run("posts the message to Resend with bearer auth", func(t *testing.T) {
		var gotAuth, gotMethod, gotPath string
		var gotBody map[string]string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sender := &ResendSender{apiKey: "test-key", from: "noreply@arkana.dev", baseURL: server.URL, httpClient: server.Client()}

		err := sender.Send(context.Background(), Message{To: "user@example.com", Subject: "Hello", HTMLBody: "<p>hi</p>"})
		if err != nil {
			t.Fatalf("Send returned error: %v", err)
		}

		if gotMethod != http.MethodPost {
			t.Errorf("method = %q, want POST", gotMethod)
		}
		if gotPath != "/emails" {
			t.Errorf("path = %q, want /emails", gotPath)
		}
		if gotAuth != "Bearer test-key" {
			t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-key")
		}
		if gotBody["from"] != "noreply@arkana.dev" {
			t.Errorf("from = %q, want %q", gotBody["from"], "noreply@arkana.dev")
		}
		if gotBody["to"] != "user@example.com" {
			t.Errorf("to = %q, want %q", gotBody["to"], "user@example.com")
		}
		if gotBody["subject"] != "Hello" {
			t.Errorf("subject = %q, want %q", gotBody["subject"], "Hello")
		}
		if gotBody["html"] != "<p>hi</p>" {
			t.Errorf("html = %q, want %q", gotBody["html"], "<p>hi</p>")
		}
	})

	t.Run("returns an error on a non-2xx response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"message": "invalid recipient"}`))
		}))
		defer server.Close()

		sender := &ResendSender{apiKey: "test-key", from: "noreply@arkana.dev", baseURL: server.URL, httpClient: server.Client()}

		err := sender.Send(context.Background(), Message{To: "bad@example.com", Subject: "Hello", HTMLBody: "<p>hi</p>"})
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})

	t.Run("wraps a transport-level failure", func(t *testing.T) {
		sender := &ResendSender{apiKey: "test-key", from: "noreply@arkana.dev", baseURL: "http://127.0.0.1:0", httpClient: http.DefaultClient}

		err := sender.Send(context.Background(), Message{To: "user@example.com", Subject: "Hello", HTMLBody: "<p>hi</p>"})
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
}

func TestNewResendSender(t *testing.T) {
	sender := NewResendSender("key", "noreply@arkana.dev")
	if sender.baseURL != resendBaseURL {
		t.Errorf("baseURL = %q, want %q", sender.baseURL, resendBaseURL)
	}
	if sender.apiKey != "key" {
		t.Errorf("apiKey = %q, want %q", sender.apiKey, "key")
	}
	if sender.from != "noreply@arkana.dev" {
		t.Errorf("from = %q, want %q", sender.from, "noreply@arkana.dev")
	}
}
