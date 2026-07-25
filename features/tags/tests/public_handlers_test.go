package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"arkana/features/tags/models"
	"arkana/features/tags/services"
)

func TestListTagsHandler(t *testing.T) {
	t.Run("returns every tag with no auth required", func(t *testing.T) {
		db := setupTestDB(t)
		svc := services.NewTagService(db)
		svc.Sync([]models.TagPayload{
			{Slug: "bitcoin", Translations: map[string]string{"en": "Bitcoin"}},
		})
		router := setupTagRouter(t, db)

		req := httptest.NewRequest("GET", "/api/tags", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
		}
		var resp models.TagListResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if len(resp.Data) != 1 || resp.Data[0].Slug != "bitcoin" {
			t.Fatalf("data = %+v, want one bitcoin entry", resp.Data)
		}
	})

	t.Run("returns an empty data array when there are no tags", func(t *testing.T) {
		db := setupTestDB(t)
		router := setupTagRouter(t, db)

		req := httptest.NewRequest("GET", "/api/tags", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var resp models.TagListResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		if resp.Data == nil {
			t.Error("data = nil, want an empty (non-nil) array")
		}
		if len(resp.Data) != 0 {
			t.Errorf("len(data) = %d, want 0", len(resp.Data))
		}
	})
}
