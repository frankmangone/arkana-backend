package models

// TagPayload is one tag's slug and its per-language display names, used
// both as the sync request's per-item shape and the public list
// response's per-item shape (translations, not columns, since the
// language set is expected to grow).
type TagPayload struct {
	Slug         string            `json:"slug"`
	Translations map[string]string `json:"translations"`
}

// TagSyncRequest is the body of POST /api/admin/tags/sync - the entire
// tags collection in one call, upserted in a single transaction.
type TagSyncRequest struct {
	Tags []TagPayload `json:"tags"`
}

type TagSyncResponse struct {
	Synced int `json:"synced"`
}

// TagResponse is one tag as returned by GET /api/tags. Same shape as
// TagPayload; kept as a distinct type since request/response payloads
// evolve independently even when they start out identical.
type TagResponse struct {
	Slug         string            `json:"slug"`
	Translations map[string]string `json:"translations"`
}

type TagListResponse struct {
	Data []TagResponse `json:"data"`
}
