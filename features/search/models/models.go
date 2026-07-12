package models

// SearchHit is a single search result, flattened from Meilisearch's response
// (its _formatted.content becomes Excerpt here), enriched with the post's
// thumbnail looked up from post_contents (Meilisearch doesn't store it).
type SearchHit struct {
	ID          string   `json:"id"`
	Lang        string   `json:"lang"`
	Path        string   `json:"path"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Excerpt     string   `json:"excerpt"`
	Thumbnail   string   `json:"thumbnail"`
}

// SearchResponse is the API response for a search request.
type SearchResponse struct {
	Query              string      `json:"query"`
	EstimatedTotalHits int         `json:"estimatedTotalHits"`
	Hits               []SearchHit `json:"hits"`
}
