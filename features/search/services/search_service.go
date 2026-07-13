package services

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"arkana/features/search/models"
)

// ErrSearchUnavailable indicates the Meilisearch backend could not be reached
// or returned an error.
var ErrSearchUnavailable = errors.New("search backend unavailable")

type SearchService struct {
	db         *sql.DB
	host       string
	masterKey  string
	httpClient *http.Client
}

func NewSearchService(db *sql.DB, host, masterKey string) *SearchService {
	return &SearchService{
		db:         db,
		host:       host,
		masterKey:  masterKey,
		httpClient: &http.Client{},
	}
}

// meiliSearchRequest mirrors the query strategy used by arkana-frontend's
// scripts/indexing/utils/meili-client.ts: crop and highlight the content
// field instead of returning it in full.
type meiliSearchRequest struct {
	Q                     string   `json:"q"`
	Limit                 int      `json:"limit"`
	AttributesToRetrieve  []string `json:"attributesToRetrieve"`
	AttributesToCrop      []string `json:"attributesToCrop"`
	CropLength            int      `json:"cropLength"`
	AttributesToHighlight []string `json:"attributesToHighlight"`
}

type meiliHit struct {
	ID          string   `json:"id"`
	Lang        string   `json:"lang"`
	Path        string   `json:"path"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Formatted   struct {
		Content string `json:"content"`
	} `json:"_formatted"`
}

type meiliSearchResponse struct {
	Hits               []meiliHit `json:"hits"`
	Query              string     `json:"query"`
	EstimatedTotalHits int        `json:"estimatedTotalHits"`
}

// Search queries the per-language Meilisearch index ("posts_<lang>") and
// returns a flattened result set.
func (s *SearchService) Search(lang, query string, limit int) (*models.SearchResponse, error) {
	reqBody := meiliSearchRequest{
		Q:                     query,
		Limit:                 limit,
		AttributesToRetrieve:  []string{"id", "lang", "path", "title", "description", "tags"},
		AttributesToCrop:      []string{"content"},
		CropLength:            20,
		AttributesToHighlight: []string{"content"},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encode search request: %w", err)
	}

	indexUID := "posts_" + lang
	url := fmt.Sprintf("%s/indexes/%s/search", s.host, indexUID)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to build search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.masterKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSearchUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: status %d: %s", ErrSearchUnavailable, resp.StatusCode, string(body))
	}

	var raw meiliSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	hits := make([]models.SearchHit, 0, len(raw.Hits))
	for _, h := range raw.Hits {
		hits = append(hits, models.SearchHit{
			ID:          h.ID,
			Lang:        h.Lang,
			Path:        h.Path,
			Title:       h.Title,
			Description: h.Description,
			Tags:        h.Tags,
			Excerpt:     h.Formatted.Content,
			Thumbnail:   s.lookupThumbnail(h.Lang, h.Path),
		})
	}

	return &models.SearchResponse{
		Query:              raw.Query,
		EstimatedTotalHits: raw.EstimatedTotalHits,
		Hits:               hits,
	}, nil
}

// lookupThumbnail joins back to post_contents by (lang, path) — the same
// unique key Meilisearch documents are built from — since Meilisearch itself
// doesn't store the thumbnail. post_contents.path includes the ".md"
// extension; Meilisearch hits don't, so it's appended here. A missing row or
// unset thumbnail just yields "" rather than failing the whole search.
func (s *SearchService) lookupThumbnail(lang, path string) string {
	var thumbnail sql.NullString

	err := s.db.QueryRow(
		"SELECT thumbnail FROM post_contents WHERE lang = ? AND path = ?",
		lang, path+".md",
	).Scan(&thumbnail)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("[Search] thumbnail lookup failed for lang=%q path=%q: %v", lang, path, err)
		}
		return ""
	}

	return thumbnail.String
}
