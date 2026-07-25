package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"arkana/features/writers/models"
)

func TestGetWriterHandler(t *testing.T) {
	t.Run("returns 200 with the full profile for a visible writer", func(t *testing.T) {
		db := setupTestDB(t)
		insertTestWriter(t, db, "frank-mangone", "Frank Mangone", true)
		router := setupRouter(t, db)

		req := httptest.NewRequest("GET", "/api/writers/frank-mangone", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.WriterResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if resp.Slug != "frank-mangone" || resp.Name != "Frank Mangone" {
			t.Errorf("slug/name = %q/%q, want frank-mangone/Frank Mangone", resp.Slug, resp.Name)
		}
	})

	t.Run("returns 404 for an unknown slug", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupRouter(t, db)

		req := httptest.NewRequest("GET", "/api/writers/nonexistent", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("returns 404 for a non-visible writer", func(t *testing.T) {
		db := setupTestDB(t)
		insertTestWriter(t, db, "hidden-writer", "Hidden Writer", false)
		router := setupRouter(t, db)

		req := httptest.NewRequest("GET", "/api/writers/hidden-writer", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

func TestListWritersHandler(t *testing.T) {
	t.Run("returns visible writers wrapped in a data key", func(t *testing.T) {
		db := setupTestDB(t)
		insertTestWriter(t, db, "anna-writer", "Anna Writer", true)
		insertTestWriter(t, db, "hidden-writer", "Hidden Writer", false)
		router := setupRouter(t, db)

		req := httptest.NewRequest("GET", "/api/writers", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.WriterListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Data) != 1 || resp.Data[0].Slug != "anna-writer" {
			t.Errorf("data = %+v, want just anna-writer", resp.Data)
		}
	})

	t.Run("returns 200 even when a legacy writer row with no slug exists", func(t *testing.T) {
		db := setupTestDB(t)
		_, err := db.Exec(`INSERT INTO writers (name, user_id) VALUES (?, ?)`, "Legacy Writer", 7)
		if err != nil {
			t.Fatal(err)
		}
		insertTestWriter(t, db, "normal-writer", "Normal Writer", true)
		router := setupRouter(t, db)

		req := httptest.NewRequest("GET", "/api/writers", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.WriterListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Data) != 1 || resp.Data[0].Slug != "normal-writer" {
			t.Errorf("data = %+v, want just normal-writer", resp.Data)
		}
	})
}
