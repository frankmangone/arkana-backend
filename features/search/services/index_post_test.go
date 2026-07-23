package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchServiceIndexPost(t *testing.T) {
	t.Run("posts a document to the per-language index", func(t *testing.T) {
		var gotPath string
		var gotBody []map[string]any

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]any{"taskUid": 1})
		}))
		defer server.Close()

		svc := NewSearchService(nil, server.URL, "master-key")
		err := svc.IndexPost("en", "cryptography-101/hashing", "Hashing 101", "desc", "content text", []string{"crypto"})
		if err != nil {
			t.Fatalf("IndexPost returned error: %v", err)
		}

		if gotPath != "/indexes/posts_en/documents" {
			t.Errorf("path = %q, want /indexes/posts_en/documents", gotPath)
		}
		if len(gotBody) != 1 {
			t.Fatalf("expected 1 document, got %d", len(gotBody))
		}
		doc := gotBody[0]
		if doc["id"] != "en-cryptography-101-hashing" {
			t.Errorf("id = %v, want en-cryptography-101-hashing", doc["id"])
		}
		if doc["title"] != "Hashing 101" {
			t.Errorf("title = %v, want %q", doc["title"], "Hashing 101")
		}
		if doc["description"] != "desc" {
			t.Errorf("description = %v, want desc", doc["description"])
		}
		if doc["content"] != "content text" {
			t.Errorf("content = %v, want %q", doc["content"], "content text")
		}
		if doc["path"] != "cryptography-101/hashing" {
			t.Errorf("path = %v, want cryptography-101/hashing", doc["path"])
		}
	})

	t.Run("returns an error on a non-2xx response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		svc := NewSearchService(nil, server.URL, "master-key")
		err := svc.IndexPost("en", "some/path", "T", "D", "C", nil)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
}
