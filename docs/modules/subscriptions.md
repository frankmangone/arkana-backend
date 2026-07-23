# Subscriptions module

Lets guests and logged-in users subscribe to be notified by email when a
new post is published, and gives an admin a way to trigger that
notification. See
[docs/superpowers/specs/2026-07-21-email-subscriptions-design.md](../superpowers/specs/2026-07-21-email-subscriptions-design.md)
for the full design rationale.

## Data model

`subscribers` table (`migrations/20260722000002_add_subscribers.sql`):

| Column            | Type      | Notes                                              |
|-------------------|-----------|-----------------------------------------------------|
| `id`              | INTEGER   | Primary key.                                         |
| `user_id`         | INTEGER   | Nullable FK to `users(id)`. NULL for a pure guest subscription. |
| `email`           | TEXT      | Unique.                                              |
| `status`          | TEXT      | One of `pending`, `confirmed`, `unsubscribed`.       |
| `created_at`      | TIMESTAMP |                                                       |
| `confirmed_at`    | TIMESTAMP | Nullable.                                            |
| `unsubscribed_at` | TIMESTAMP | Nullable.                                            |

`posts.title` lives in `post_contents.title`
(`migrations/20260722000001_add_post_contents_title.sql`), written by
`arkana-frontend/scripts/publish-content.js` from the source file's
frontmatter `title` field.

## Config

| Env var                    | Required | Purpose                                                     |
|-----------------------------|----------|---------------------------------------------------------------|
| `RESEND_API_KEY`            | For sending to work | Resend API key.                                    |
| `RESEND_FROM_EMAIL`         | For sending to work | The `from` address used for all subscription emails. |
| `SUBSCRIPTION_TOKEN_SECRET` | Yes      | HMAC secret for confirm/unsubscribe tokens. Keep private — anyone with it can forge these tokens for any subscriber id. |
| `ADMIN_HMAC_SECRET`         | Yes      | HMAC secret for the admin broadcast endpoint. Keep private — anyone with it can trigger broadcasts. |
| `FRONTEND_URL`              | No (defaults to `http://localhost:5173`) | Base URL used to build confirm/unsubscribe links embedded in emails. |

## Endpoints

### `POST /api/subscribe` — guest signup (public)

Request:
```json
{ "email": "person@example.com" }
```

Response (always this shape, `200`, regardless of what happened internally):
```json
{ "status": "pending" }
```

Behavior:
- New or previously-unsubscribed email → creates/resets a `pending` row and sends a confirmation email.
- Already-`confirmed` email → no-op, same response.
- **Email matches an existing `users` account → silently no-ops** (no row created, no email sent, a warning is logged server-side). This is intentional: the response must never disclose whether an email belongs to a registered account.
- `400` if `email` is missing.
- `500` if the confirmation email fails to send (the row still exists as `pending` so retrying `POST /api/subscribe` will resend it).

### `POST /api/subscribe/confirm` — confirm a guest signup (public)

Request:
```json
{ "subscriber_id": 1, "token": "..." }
```

Response: `{ "confirmed": true }`. Idempotent — confirming an already-`confirmed` row, or replaying a stale confirm link against a row that has since been `unsubscribed`, is a no-op success (it will **not** resurrect an unsubscribed row).

`400` for an invalid, tampered, or wrong-purpose token.

### `POST /api/subscriptions` — subscribe as the logged-in user (JWT auth)

`Authorization: Bearer <access token>`. No body needed.

Response: `{ "subscribed": true }`. Subscribes immediately as `confirmed` (no confirm step — the account email is already OAuth-verified). If a guest row already exists for the account's email, it gets linked (`user_id` set) and confirmed rather than duplicated. Idempotent.

`401` if unauthenticated.

### `DELETE /api/subscriptions` — unsubscribe as the logged-in user (JWT auth)

Response: `{ "subscribed": false }`. No-op if the user was never subscribed. Idempotent.

`401` if unauthenticated.

### `POST /api/subscriptions/unsubscribe` — one-click unsubscribe (public)

Request:
```json
{ "subscriber_id": 1, "token": "..." }
```

Response: `{ "unsubscribed": true }`. Works uniformly for guest and user-linked rows. Idempotent. `400` for an invalid or wrong-purpose token.

### `POST /api/admin/subscriptions/broadcast` — notify all subscribers of a new post (HMAC auth)

Headers:
- `X-Timestamp`: unix seconds.
- `X-Signature`: `hex(HMAC-SHA256(ADMIN_HMAC_SECRET, X-Timestamp + "." + rawRequestBody))`.

Request:
```json
{ "post_id": 123 }
```

Response: `{ "sent": 42, "failed": 1 }`. A failure sending to one recipient is logged and does not abort the batch.

- `401` (generic message) if the signature is missing/invalid or the timestamp is more than 5 minutes old — doesn't reveal which check failed.
- `404` if the post has no content row.

Example signing (bash):
```bash
body='{"post_id":123}'
ts=$(date +%s)
sig=$(printf '%s.%s' "$ts" "$body" | openssl dgst -sha256 -hmac "$ADMIN_HMAC_SECRET" | sed 's/^.* //')
curl -X POST https://<host>/api/admin/subscriptions/broadcast \
  -H "X-Timestamp: $ts" -H "X-Signature: $sig" -H "Content-Type: application/json" \
  -d "$body"
```

## Token scheme

Confirm and unsubscribe tokens are stateless HMAC, not stored in the DB:

```
token = hex(HMAC-SHA256(SUBSCRIPTION_TOKEN_SECRET, "<subscriber_id>.<purpose>"))
```

`purpose` is `confirm` or `unsubscribe` — baked into the signature so a confirm-email link can't be replayed to unsubscribe someone (or vice versa). Tokens don't expire.

## Not implemented (v1 non-goals)

- Automatic send-on-publish (broadcast is always a manual/scripted call after publishing).
- Digest/batched emails — one email per post, sent once.
- Per-subscriber preferences (topics, languages, frequency).
- Arbitrary admin-composed broadcast emails (content is always the post-notification template).
- Frontend UI (subscribe form, confirm/unsubscribe landing pages) — backend only so far.
