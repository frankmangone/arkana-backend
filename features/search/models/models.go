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

// TagHit is a single tag with the number of posts carrying it.
type TagHit struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// TagSearchResponse is the API response for a tag type-ahead request,
// ordered by post count (most-used tags first).
type TagSearchResponse struct {
	Query string   `json:"query"`
	Tags  []TagHit `json:"tags"`
}

// SearchResponse is the API response for a search request.
// FacetDistribution is only present when facets were requested; it maps
// facet name -> value -> count within the current result set
// (e.g. {"tags": {"plonk": 2}}).
type SearchResponse struct {
	Query              string                    `json:"query"`
	EstimatedTotalHits int                       `json:"estimatedTotalHits"`
	Hits               []SearchHit               `json:"hits"`
	FacetDistribution  map[string]map[string]int `json:"facetDistribution,omitempty"`
}
