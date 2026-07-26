package models

// Translation is one language's title/description, used at both the
// reading-list and module level.
type Translation struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// ItemPayload's Slug is the item's own short identifier (e.g.
// "how-it-all-began"); PostPath is the post it points to (e.g.
// "blockchain-101/how-it-all-began"), validated against
// posts.path_identifier at publish time. Order is a per-module sort key.
type ItemPayload struct {
	Slug     string `json:"slug"`
	PostPath string `json:"post_path"`
	Order    int    `json:"order"`
}

type ModulePayload struct {
	Slug         string                 `json:"slug"`
	Order        int                    `json:"order"`
	Translations map[string]Translation `json:"translations"`
	Items        []ItemPayload          `json:"items"`
}

// ReadingListPayload is the body of POST /api/admin/reading-lists - one
// reading list's complete structure, replacing whatever was previously
// published for this Slug.
type ReadingListPayload struct {
	Slug         string                 `json:"slug"`
	CoverImage   string                 `json:"cover_image,omitempty"`
	Ongoing      bool                   `json:"ongoing"`
	Translations map[string]Translation `json:"translations"`
	Modules      []ModulePayload        `json:"modules"`
}

type PublishReadingListResponse struct {
	Published bool `json:"published"`
}

// Response shapes mirror the payload shapes exactly (same rationale as
// tags.TagResponse/TagPayload: kept distinct types since request/response
// evolve independently even though they start out identical).
type ItemResponse struct {
	Slug     string `json:"slug"`
	PostPath string `json:"post_path"`
	Order    int    `json:"order"`
}

type ModuleResponse struct {
	Slug         string                 `json:"slug"`
	Order        int                    `json:"order"`
	Translations map[string]Translation `json:"translations"`
	Items        []ItemResponse         `json:"items"`
}

type ReadingListResponse struct {
	Slug         string                 `json:"slug"`
	CoverImage   string                 `json:"cover_image,omitempty"`
	Ongoing      bool                   `json:"ongoing"`
	Translations map[string]Translation `json:"translations"`
	Modules      []ModuleResponse       `json:"modules"`
}

type AdminReadingListListResponse struct {
	Data []ReadingListResponse `json:"data"`
}
