# Seed strategy — busy realistic demo + live-user simulator

Date: 2026-08-19
Status: brainstorm → design doc (not yet implemented)

## Context / problem

A production deployment (`deploy/apply.sh` on EC2) runs migrations on api boot
(`server/scripts/docker-entrypoint.sh`) but **never runs the seed binary**, so a
fresh EC2 database comes up with schema only and zero content — exactly what
http://100.31.118.41/ shows right now.

The current seed (`server/cmd/seed/main.go`) exists and is:
- idempotent (exits early when `alice@example.com` exists),
- built into the api image (`/app/seed`, `server/Dockerfile`),
- but only invoked manually via `make seed` (`docker compose run --rm --no-deps
  --entrypoint /app/seed api`).
- tiny: 8 users, 16 posts + 8 replies, follows, 1 block, 4 **orphan media rows**
  (no file written to disk, not attached to any post/profile).

## Goals

1. A fresh deploy (local `make dev` AND EC2 prod) comes up already showing a
   busy, realistic community with **no extra manual step**.
2. Dataset feels real: enough users/posts/engagement that feeds, trends, search,
   DMs, lists, polls, badges all look populated; content spread over the past
   ~4 weeks so feeds/trends age naturally instead of a timestamp burst.
3. Structure the generators so a future "live users" cron (scheduled on the EC2
   box) can reuse the same primitives — seeded users acting over time.
4. Deterministic + idempotent: re-running any seed step is safe and repeatable.

## Non-goals (for now)

- Superset of today's UX — no new user-facing features.
- Production "real" user path beyond the anchor/demo accounts.
- The cron itself: this doc only plans the seam; implementation is a later task.

## Approach decisions (brainstorm-resolved)

### Where to seed → api entrypoint (`SEED_ON_START`)

Hook seeding into the same place migrations already run:

`server/scripts/docker-entrypoint.sh` gains, after the migrate loop and before
`exec /app/api`:

```sh
if [ "${SEED_ON_START:-false}" = "true" ]; then
  echo ">> running seed (SEED_ON_START=true)"
  /app/seed
fi
```

- The seed is already idempotent, so every boot after the first is a no-op
  (guarded by `alice@example.com` existing) — a single fast SELECT.
- `compose.yaml` sets `SEED_ON_START: "true"` so **both** local dev and prod
  (compose.prod.yaml inherits api env) auto-seed on first boot; re-deploys are
  no-ops.
- Alternative rejected: adding an explicit step to `deploy/apply.sh` (option B)
  — works for EC2 only, leaves local dev manual, and adds deploy-script
  coupling. `SEED_ON_START` covers all environments through one switch.
- Alternative rejected: SQL/`psql` seed script (option C) — content-in-SQL is
  unmaintainable, can't model DMs/lists/polls well, and shares no primitives
  with the future cron.

### How to generate content → faker library (`brianvoe/gofakeit`)

- Add `github.com/brianvoe/gofakeit/v7` to `server/go.mod` for realistic
  names/usernames/bios/post bodies/location data.
- Always drive the generator from a fixed seed (`gofakeit.New(int64)`) so the
  dataset is **deterministic** across environments and re-runs.
  - Note: even with a fixed seed, IDs/timestamps vary (identity columns,
    `now()`), so row content is stable but not byte-identical across DBs.
  - Trade-off accepted: reproducibility of *shapes* (same users, same feeds,
    same counts structure) matters more than exact equality.

### Backdating posts → needs a store extension (found constraint)

`post_store.go:261`: `INSERT INTO posts (content, author_id, parent_id,
visibility, mentioned_user_ids)` — `created_at` isn't settable through the
store (DB default `now()`). Spread the seed's posts across the last ~4 weeks in
one of two ways:

1. Extend `post_store.Create`/`CreateWithQuote` with an optional
   `createdAt *time.Time` (nil → `now()`), writing it in the INSERT. Smallest
   blast radius; keeps the write path used by the app intact.
2. A dedicated seed-only raw-SQL insert path in `internal/seedgen`.

**Option 1 (CHOSEN) — honor `post.CreatedAt` in the store via `COALESCE`:**

`models.Post` already carries `CreatedAt time.Time` (post.go:42), so no model
change. Change both INSERTs (`post_store.go:260-264` Create and `:289-293`
CreateQuotedPost) to include `created_at, updated_at` and backfill via
`COALESCE`:

```sql
INSERT INTO posts (content, author_id, parent_id, visibility, mentioned_user_ids, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, COALESCE($6, CURRENT_TIMESTAMP), CURRENT_TIMESTAMP)
RETURNING post_id, created_at, updated_at
```

Pass `nil` when `post.CreatedAt.IsZero()`, else the time:

```go
var createdAt any
if !post.CreatedAt.IsZero() {
	createdAt = post.CreatedAt
}
err := exec(ctx, query, post.Content, post.AuthorID, post.ParentID,
	post.Visibility, pq.Array(nonNilIntSlice(post.MentionedUserIDs)), createdAt).
	Scan(&post.ID, &post.CreatedAt, &post.UpdatedAt)
```

Safety: the only app call sites (`post_service.go:467` and `:1031`) build fresh
`post` values that never set `CreatedAt` → always zero → `NULL` →
`CURRENT_TIMESTAMP`, exactly today's behavior. Only the seed sets it, and the
`RETURNING` scanner already copies the value back into `post.CreatedAt`. No
migration, no model change, both create paths covered.

Option 2 (seed-only raw SQL insert in `seedgen`, fallback) would duplicate the
insert schema in seedgen and bypass store invariants — only considered if
touching the store is ever unwanted.

Engagement writes (likes / reposts / bookmarks via `post_engagement_store.go`)
and followers already record their own `created_at`; those need the same
optional-timestamp treatment if we want engagement arbitrary-backdated, but
seeding engagement mostly "after" the post timestamp is sufficient and simpler.

### What the seed creates

Scale target (busy but not pathological; adjust via consts/flags):

- **30 users** (via faker) + the current deterministic anchor users
  (alice/admin, bob, charlie, …) kept stable for login/testing.
- **~400 top-level posts** spread across the last 28 days + **~150 replies** on
  a subset, nested where sensible.
- **Engagement** per post, biased after the post's timestamp:
  - likes (0–25), reposts (0–8), bookmarks (0–12)
  - polls on ~10% of posts (2–4 options, votes spread over followers)
  - mentions (`mentionStore.SyncPost`) in ~15% of posts
  - rich hashtags on ~40% (so `/trends` and hashtag feeds are alive)
  - reply-threads including at least one @-mention reply
- **Relationships**: follows (dense web, every user follows ~8–15), a couple of
  blocks and mutes, private accounts for ~3 users + `followers`-visibility posts
  to exercise privacy enforcement.
- **DMs**: ~8–12 conversations with messages spread over days (both directions).
- **Lists**: ~6 lists, memberships.
- **Badges**: grant a few assigned badges; earned ones compute automatically.
- **Media**: replace the orphan-media behavior — generate small placeholder
  images and WRITE them to `MEDIA_DIR` (`<uuid>` filename) and attach via
  `post_media` / profile picture/banner so `<img>`/`GET /media/{uuid}` resolve.
  Stdlib `image/png` (solid/gradient blobs) — no new dependency.
  - Posting media currently goes through `SaveFile` + `Create` + `AttachToPost`
    which seeds ~12–20 posts with a single image each.

### Generator architecture (the seam the cron reuses)

New package `server/internal/seedgen`:

- `seedgen.Generate(rng) *Dataset` — a pure in-memory model of users, posts,
  relationships, DMs, lists, badges, media specs, with no DB writes. Deterministic
  given `rng`.
- `seedgen.Apply(ctx, store, dataset, opts)` — writes it to the DB via the
  stores, honoring existing service-layer invariants that matter:
  hashtag sync, mention sync, engagement counters.
- `cmd/seed` (bulk initial load) = `Generate` + `Apply` + keep the
  `alice@example.com` idempotency guard.
- Future `cmd/simulate` (one activity tick) = same `Generate` machinery but a
  `Tick(rng, store, existingUsers)` that creates a few fresh posts/likes/
  replies/DMs **using `now()` timestamps** so the population grows over time
  like real users. The design leaves a single seam: a package-level function
  that performs one "user action cycle", which both bulk seed and the ticker
  call.

### The future cron (seam only, not implemented here)

On the EC2 box, schedule `cmd/simulate` via a systemd timer or host crontab:

```
docker compose -f /srv/gaggle/compose.yaml -f /srv/gaggle/compose.prod.yaml \
  run --rm --no-deps --entrypoint /app/simulate api
```

- Reuses the already-built api image; no new service needed.
- Runs against the internal compose network; acts as the seeded users only.
- Kept OUT of `SEED_ON_START`/settle path so normal api boots never simulate.
- Local dev analogue: `make simulate`.
- (Github Actions deploy creates no infrastructure for this; the instance hosts
  it directly, matching "that might go on the aws instance itself".)

### AWS deploy consideration

- Fresh EC2 EBS = empty `postgres_data` volume → first `up -d` migrates + seeds.
- Re-deploys keep the volume → seed exits early (idempotent). No `apply.sh`
  change required for seeding.
- If we ever want prod to stay clean of fake users, toggle `SEED_ON_START=false`
  via `/srv/gaggle/.env` and run seed explicitly.

## Files touched (impl plan sketch)

- `server/go.mod` / `go.sum` — add `brianvoe/gofakeit/v7`.
- `server/internal/seedgen/` — new package (generate + apply + tick seam).
- `server/cmd/seed/main.go` — rewrite to use seedgen, keep anchor users +
  idempotency guard.
- `server/cmd/simulate/main.go` — new (future cron binary; scaffold can be part
  of this work so the seam is proven).
- `server/internal/store/post_store.go` — optional createdAt param (or raw-SQL
  seed path if that's preferred).
- `server/cmd/migrate/migrations/` — expects **no** new migration (schema
  unchanged); renumber nothing.
- `server/scripts/docker-entrypoint.sh` — `SEED_ON_START` hook.
- `compose.yaml` — `SEED_ON_START: "true"` on api.
- `Makefile` — `make simulate` + update `make seed` help text.
- `README.md` — seed section refresh (auto-seed instead of manual).

## Testing

- Extend/integration tests in `server/internal/seedgen` covering:
  - `Generate`/`Apply` produce the scale and shapes above;
  - engagement counters, hashtag/mention sync, follower counts correct after
    apply;
  - idempotency: applying twice yields approximately the same state, no dup
    users (guard on alice).
  - `created_at` backdating lands within the intended window.
- Existing `make test` (throwaway `social_test` DB) must stay green — the seed
  is not in the test boot path.
- Manual/local: `make dev` on a wiped volume → auto-seeded site; `/trends`,
  search, DMs, lists, badges all visibly populated; media URLs resolve.

## Reviewer double-checks

- `post_store.Create` optional-`created_at` change must not regress the app
  write path; confirm the sim/bulk path is the only one passing a non-nil value.
- Verify `SEED_ON_START=true` in `compose.yaml` doesn't slow every api boot
  meaningfully (it must exit early when alice exists).
- Confirm no new migration version was introduced (avoids parallel-branch
  migration collisions).
- Confirm seed does not write into Redis caches in a way that leaves stale home
  feeds (60s TTL makes this a non-issue, but worth double-checking the feed
  invalidation path is what the app itself uses).
- Media placeholder generation must write files that exist and are valid PNGs
  under `MEDIA_DIR`; `GET /media/{uuid}` returns 200.
- The simulator seam (`Tick`) shared with bulk seed — flag if the two want to
  diverge.