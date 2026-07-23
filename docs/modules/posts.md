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
- The body (frontmatter stripped) is stored in `post_contents.content`.
- The body is also markdown-stripped (headings/bold/italic/links/code
  fences/etc. removed, LaTeX left untouched) and indexed into Meilisearch
  alongside `title`/`description`/`tags`.

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
