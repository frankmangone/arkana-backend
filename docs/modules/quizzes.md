# Quizzes module

Admins publish individual `questions`; there is no persisted "quiz"
entity — a quiz for a given reading-list module is assembled dynamically
at attempt-start time by querying the question pool through
`question_posts` → `posts` → `reading_list_items`. See
[docs/superpowers/specs/2026-07-29-quiz-data-model-design.md](../superpowers/specs/2026-07-29-quiz-data-model-design.md)
for the full design rationale, including why content links to `posts`
directly rather than to reading-list items.

## Data model

`questions` (`migrations/20260729000001_add_quizzes.sql`):

| Column       | Type      | Notes                                                              |
|--------------|-----------|---------------------------------------------------------------------|
| `id`         | INTEGER   | Primary key, internal only.                                         |
| `uuid`       | TEXT      | Unique. Opaque public identifier — the only id ever sent to a client.|
| `slug`       | TEXT      | Unique. Admin-authored upsert key for `POST /api/admin/questions`.   |
| `type`       | TEXT      | `single_choice`, `multi_choice`, `matching`, `range`, `sequencing`, `bucket_sort`, `fill_blank`. No CHECK constraint — a new type needs no migration. |
| `difficulty` | INTEGER   | Meaning defined by a shared backend/frontend constant (1=Easy, 2=Medium, 3=Hard). |
| `answer_key` | TEXT      | JSON, language-agnostic, **never sent to the client**.               |

`question_translations` — one row per `(question_id, lang)`: `prompt`
(TEXT) and `content` (JSON, translatable display data; may carry an
optional `explanation` string shown on incorrect reveal).

`question_tags` — `(question_id, tag_id)` pivot into the existing `tags`
vocabulary.

`question_posts` — `(question_id, post_id)` pivot into `posts`,
**many-to-many, no uniqueness beyond the compound key**. A question can
link more than one post; reinforcement after a wrong/skipped answer
surfaces every linked post, not a single "primary" one.

`quiz_attempts`, `quiz_attempt_questions`, and `quiz_attempt_answers` are
**not** SQL tables - attempt/session state is ephemeral and lives in
Redis instead, one JSON blob per attempt at `quiz:attempt:{uuid}` plus a
small resume-index key `quiz:active-attempt:{userID}:{moduleID}`. Both
keys carry a 2-hour TTL that slides forward on every read or write
(`Start`, `Next`, `Answer`, `Complete` all touch it) - an actively-used
attempt never expires mid-session, while an abandoned one cleans itself
up 2 hours after the last interaction. See
[docs/superpowers/specs/2026-08-03-quiz-attempts-redis-design.md](../superpowers/specs/2026-08-03-quiz-attempts-redis-design.md)
for the full design.

## Config

Reuses `ADMIN_HMAC_SECRET` (same admin HMAC middleware as
`posts`/`tags`/`readinglists`/`writers` — see
[docs/modules/subscriptions.md](subscriptions.md#config)) for
`POST /api/admin/questions`, and the existing JWT setup from
`features/auth` for every session route. No new env vars.

Two implementation constants live in `features/quizzes/services` as Go
`const`, not config: `questionsPerAttempt = 8` (how many questions
`WeightedRandomSelector` draws into one attempt) and
`passThreshold = 0.7` (score fraction required to pass).

## `POST /api/admin/questions`

Headers (same HMAC scheme as every other admin endpoint):
- `X-Timestamp`: unix seconds.
- `X-Signature`: `hex(HMAC-SHA256(ADMIN_HMAC_SECRET, X-Timestamp + "." + rawRequestBody))`.

Request:
```json
{
  "questions": [
    {
      "slug": "pow-difficulty-basics",
      "type": "single_choice",
      "difficulty": 1,
      "post_paths": ["blockchain-101/how-it-all-began"],
      "tags": ["cryptography"],
      "answer_key": { "correctOptionIds": ["b"] },
      "translations": {
        "en": { "prompt": "...", "content": { "options": [] } }
      }
    }
  ]
}
```

Response: `{"published": 1}`.

Add/update only, upserted by `slug` — a question omitted from a later
payload is never deleted (avoids orphaning an in-flight or historical
attempt that already answered it). Every `post_paths` entry must already
correspond to a published post, and every tag must already exist — the
backend rejects the entire batch (400, naming what's missing) before
writing anything if either check fails.

- `401` (generic message) on an invalid/missing signature or a timestamp more than 5 minutes old.
- `400` if any question has an empty `slug`/`type`, or references an
  unknown post path or tag.

## `POST /api/reading-lists/{listSlug}/modules/{moduleSlug}/quiz/attempts`

JWT auth (`Authorization: Bearer <access token>`). No body.

Response:
```json
{ "attemptId": "<uuid>", "totalQuestions": 8 }
```

Runs `WeightedRandomSelector` once against every question linked (via
`question_posts`) to a post referenced by any item in this module, and
persists the pick-order — there is deliberately **no question body in
this response**; the client always fetches the current question via
`GET .../next`, whether it's the first or the last.

- `401` if unauthenticated.
- `404` if the list/module slug doesn't resolve to a real module.

## `GET /api/quiz-attempts/{attemptId}/next?lang=en`

JWT auth, plus an explicit ownership check (`quiz_attempts.user_id` must
equal the authenticated user) — the opaque `attemptId` is
defense-in-depth, never a substitute for this check.

Response:
```json
{
  "question": {
    "uuid": "<question-uuid>",
    "type": "single_choice",
    "difficulty": 1,
    "prompt": "...",
    "content": { "options": [] }
  },
  "position": 0,
  "totalQuestions": 8,
  "done": false
}
```

The returned question has every correct-answer field stripped. Calling
this endpoint repeatedly **without** an intervening `answers` call
returns the identical question every time — nothing here advances state,
only a graded or skipped answer does. Once every position has been
resolved, returns `{"question": null, "done": true, ...}`.

- `401` if unauthenticated.
- `404` if the attempt doesn't exist **or** belongs to a different user
  (same response either way — a non-owner must never learn that an
  attempt uuid they don't own actually exists).
- `409` if the attempt is already completed.
- `400` if `lang` is present but unsupported (`en`/`es`/`pt`).

## `POST /api/quiz-attempts/{attemptId}/answers?lang=en`

JWT auth + ownership check, same as `next`. Body is exactly one of:

```json
{ "questionId": "<uuid>", "response": { "selectedOptionIds": ["b"] } }
```
```json
{ "questionId": "<uuid>", "skipped": true }
```

Per-type `response` / `correctReveal` shapes:

| type | `response` | `correctReveal` |
|---|---|---|
| `single_choice` / `multi_choice` | `{ "selectedOptionIds": string[] }` | `{ "correctOptionIds": string[] }` |
| `matching` / `bucket_sort` | `{ "assignments": { [id]: id } }` | `{ "correctAssignments": { [id]: id } }` |
| `range` | `{ "values": { [rangeId]: number } }` | `{ "correctValues": { [rangeId]: { value, tolerance } } }` |
| `sequencing` | `{ "order": string[] }` | `{ "correctOrder": string[] }` |
| `fill_blank` | `{ "filled": { [blankId]: string } }` | `{ "correctWords": { [blankId]: string } }` |

Response:
```json
{
  "correct": false,
  "skipped": false,
  "correctReveal": { "correctOptionIds": ["b"] },
  "explanation": "explanation text here",
  "reinforcement": { "postPaths": ["blockchain-101/how-it-all-began"] },
  "attemptDone": false
}
```

`correctReveal` is populated whenever `correct` is `false` (wrong or skipped alike; skipping still teaches you the right answer).
`explanation` is present only when the question's content has explanation text AND the answer wasn't correct.
`reinforcement` is present only when the question has linked posts AND the answer wasn't correct.
All three are omitted entirely (keys not included) on a correct answer, following Go's `json:"...,omitempty"` tag behavior.
A skip is graded `correct: false` with no third scoring branch; `skipped` is metadata only.

Rejects (409) if `questionId` doesn't match the question actually at the
attempt's current position — this, plus a uniqueness constraint on
`(attempt_id, question_id)`, is what stops answering out of order or
twice.

- `401`/`404` — same as `next`.
- `409` — attempt already completed, or `questionId` mismatch.
- `400` — body doesn't contain exactly one of `response`/`skipped`, or
  unsupported `lang`.

## `POST /api/quiz-attempts/{attemptId}/complete`

JWT auth + ownership check. No body.

Response: `{ "score": 50, "passed": false }`.

Requires every question in the attempt to already have a row in
`quiz_attempt_answers` (answered or skipped, either counts as
"resolved") — otherwise 400. The client always knows exactly when it's
reached this point from the last `answers` call's `attemptDone: true`, so
a premature `complete` call is treated as a client bug, not a state this
endpoint papers over.

- `401`/`404` — same as `next`.
- `409` — attempt already completed.
- `400` — not every question has been answered or skipped yet.

## Not implemented (v1 non-goals)

- Payment rails / real monetization gating — `CanAttempt` is a stub that
  always returns `true`; the counting logic and call site exist for a
  future free/paid gate to slot into.
- Certificate tier issuance/verification — `quiz_attempts.tier` is a
  placeholder column only.
- Adaptive question selection — `WeightedRandomSelector` is the only
  `QuestionSelector` implementation; the interface exists for a future
  `AdaptiveSelector` to use `AnsweredQuestion.Correct`.
- Frontend wiring to these endpoints — the sandbox components built in
  `arkana-frontend/src/features/quiz/components` assume correctness data
  is available on the `Question` object itself (fine for local fixtures);
  wiring them to this API means moving that assumption to the
  `answers` response instead, not yet done.
- A persisted `quizzes` schema, if a future requirement makes "quiz" a
  real entity instead of a dynamic per-module query.
