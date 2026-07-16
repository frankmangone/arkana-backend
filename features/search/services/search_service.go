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
	"strconv"
	"strings"

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
	Filter                string   `json:"filter,omitempty"`
	Facets                []string `json:"facets,omitempty"`
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
	Hits               []meiliHit                `json:"hits"`
	Query              string                    `json:"query"`
	EstimatedTotalHits int                       `json:"estimatedTotalHits"`
	FacetDistribution  map[string]map[string]int `json:"facetDistribution"`
}

// SearchParams are the inputs to Search. The handler guarantees that Query
// and Tags are not both empty; with an empty Query, Meilisearch performs a
// placeholder search returning every document matching the filter — that is
// what pure tag browsing relies on.
type SearchParams struct {
	Lang     string
	Query    string
	Tags     []string
	MatchAll bool // true: posts must carry every tag; false: any of them
	Facets   bool // include tag counts for the result set
	Limit    int
}

// buildTagFilter renders a Meilisearch filter expression over the "tags"
// attribute. Values are quoted so a tag can never break out of the
// expression.
func buildTagFilter(tags []string, matchAll bool) string {
	quoted := make([]string, len(tags))
	for i, tag := range tags {
		quoted[i] = strconv.Quote(tag)
	}

	if matchAll {
		parts := make([]string, len(quoted))
		for i, q := range quoted {
			parts[i] = "tags = " + q
		}
		return strings.Join(parts, " AND ")
	}

	return "tags IN [" + strings.Join(quoted, ", ") + "]"
}

// Search queries the per-language Meilisearch index ("posts_<lang>") and
// returns a flattened result set.
func (s *SearchService) Search(params SearchParams) (*models.SearchResponse, error) {
	reqBody := meiliSearchRequest{
		Q:                     params.Query,
		Limit:                 params.Limit,
		AttributesToRetrieve:  []string{"id", "lang", "path", "title", "description", "tags"},
		AttributesToCrop:      []string{"content"},
		CropLength:            20,
		AttributesToHighlight: []string{"content"},
	}

	if len(params.Tags) > 0 {
		reqBody.Filter = buildTagFilter(params.Tags, params.MatchAll)
	}
	if params.Facets {
		reqBody.Facets = []string{"tags"}
	}

	url := fmt.Sprintf("%s/indexes/posts_%s/search", s.host, params.Lang)

	var raw meiliSearchResponse
	if err := s.postMeili(url, reqBody, &raw); err != nil {
		return nil, err
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
		FacetDistribution:  raw.FacetDistribution,
	}, nil
}

type meiliFacetSearchRequest struct {
	FacetName  string `json:"facetName"`
	FacetQuery string `json:"facetQuery,omitempty"`
}

type meiliFacetSearchResponse struct {
	FacetHits []struct {
		Value string `json:"value"`
		Count int    `json:"count"`
	} `json:"facetHits"`
}

// SearchTags runs a facet search over the "tags" attribute of the
// per-language index: a type-ahead over tag values with post counts,
// ordered by count (the index's sortFacetValuesBy setting). An empty query
// returns the most-used tags.
func (s *SearchService) SearchTags(lang, query string) (*models.TagSearchResponse, error) {
	reqBody := meiliFacetSearchRequest{
		FacetName:  "tags",
		FacetQuery: query,
	}

	url := fmt.Sprintf("%s/indexes/posts_%s/facet-search", s.host, lang)

	var raw meiliFacetSearchResponse
	if err := s.postMeili(url, reqBody, &raw); err != nil {
		return nil, err
	}

	tags := make([]models.TagHit, 0, len(raw.FacetHits))
	for _, hit := range raw.FacetHits {
		tags = append(tags, models.TagHit{Tag: hit.Value, Count: hit.Count})
	}

	return &models.TagSearchResponse{Query: query, Tags: tags}, nil
}

// postMeili sends a JSON payload to a Meilisearch endpoint and decodes the
// 200 response into out; any transport or non-200 failure is wrapped in
// ErrSearchUnavailable where appropriate.
func (s *SearchService) postMeili(url string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.masterKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSearchUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: status %d: %s", ErrSearchUnavailable, resp.StatusCode, string(respBody))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
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
