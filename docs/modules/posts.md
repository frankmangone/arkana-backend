# Posts module — admin publish endpoint

Documents the CI-facing addition to the `posts` module: a single
HMAC-authenticated endpoint that publishes a post from raw markdown,
called by the `arkana-content` repo's `publish.yml` GitHub Action.

## Config

Reuses the same `ADMIN_HMAC_SECRET` as the subscriptions broadcast
endpoint (see
[docs/modules/subscriptions.md](subscriptions.md#config)) — one shared
admin HMAC middleware, one secret, for both admin endpoints.

## `POST /api/admin/posts`

Headers (same HMAC scheme as the broadcast endpoint):
- `X-Timestamp`: unix seconds.
- `X-Signature`: `hex(HMAC-SHA256(ADMIN_HMAC_SECRET, X-Timestamp + "." + rawRequestBody))`.

Request:
```json
{
  "path": "cryptography-101/hashing",
  "lang": "en",
  "raw_content": "---\ntitle: Hashing 101\n---\n\n# Hashing\n\nPost body..."
}
```

`raw_content` is the whole markdown file as-is — YAML frontmatter between
`---` delimiters, followed by the body. All content processing happens
server-side:
- Frontmatter is parsed for `title`, `thumbnail`, `description`, `tags`.
- The full raw content — frontmatter included — is stored verbatim in
  `post_contents.content`, since `pull-content.js` writes this column's
  value straight out to disk as the served `.md` file.
- Separately, and only for the search index, the body (frontmatter
  stripped) is also markdown-stripped (headings/bold/italic/links/code
  fences/etc. removed, LaTeX left untouched) and indexed into Meilisearch
  alongside `title`/`description`/`tags`. This stripped copy is never
  persisted — it only ever reaches Meilisearch.

This keeps the calling CI workflow dependency-free (`jq`/`openssl`/`curl`
only) — no markdown/YAML parsing logic needs to live outside the backend.

Response: `{"published": true}`.

Re-publishing the same `path`/`lang` updates the existing `post_contents`
row rather than creating a duplicate. A `path` used across multiple
languages shares the same underlying `posts` row.

- `401` (generic message) on an invalid/missing signature or a timestamp
  more than 5 minutes old.
- `400` if `path`, `lang`, or `raw_content` is missing.
- `500` on a DB or Meilisearch failure (frontmatter parse errors and
  indexing failures both count as failures — nothing is left half-written
  from the caller's perspective, though the `posts` row itself is
  idempotent to retry).

## `GET /api/admin/posts`

Paginated content-pull endpoint used by the `arkana-frontend` build
pipeline's `scripts/pull-content.js` to pull all published content over
HTTP instead of SSH+sqlite3.

Headers (same HMAC scheme as `POST /api/admin/posts`, but computed over an
empty body since a GET request has none):
- `X-Timestamp`: unix seconds.
- `X-Signature`: `hex(HMAC-SHA256(ADMIN_HMAC_SECRET, X-Timestamp + "."))` —
  i.e. `rawRequestBody` is the empty string.

Query params:
- `limit` — default `20`. Values above `100` are clamped down to `100`
  rather than rejected. Rejected with `400` if `<1` or non-numeric.
- `offset` — default `0`. Rejected with `400` if negative or non-numeric.

Only rows with `visible = 1` in `post_contents` are ever returned.

Response:
```json
{
  "data": [
    { "lang": "en", "path": "cryptography-101/hashing.md", "content": "---\ntitle: Hashing 101\n---\n\n# Hashing\n\nPost body..." }
  ],
  "total": 42
}
```

`content` is the full raw markdown verbatim, frontmatter included — the
same value `Publish` stored in `post_contents.content` (see above). `total`
is the total count of visible rows regardless of the current page's
`limit`/`offset`, so the caller knows when to stop paging.

Since the uniqueness constraint on `post_contents` is `(lang, path)` and
not `path` alone, the same `path` can legitimately appear more than once
in the combined result set across pages — once per language it's been
published in.
