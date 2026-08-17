# Project notes: GopherSocial

Tricky things learned while working on this repo (newest on top).

## API contract
- **Every** response is `{"data": ..., "error": null | {code, message}}`. The single
  success-path helper `internal/util/json.go:RespondWithJson` wraps everything, so
  handlers must NOT double-wrap (see the old Quote handler bug).
- Paginated feeds use **`items`** (not `posts`/`followers`), plus `next_cursor`/`has_more`.
- Post responses expose counts + per-viewer flags under `engagement` (`internal/models/post.go`).
  The flat count fields on `Post` are `json:"-"`; the service hydrates `Engagement`
  per viewer via `store.PostEngagements.GetEngagementForPosts`.
- `GET /media/{uuid}` is deliberately public (no auth) because `<img>` can't send the
  Authorization header; UUIDs are unguessable.
- Settings: stored as JSONB in `user_settings` (migration 000008). PATCH handler loads
  current settings, then decodes the patch into them, then saves (merge-on-read).

## DB / schema gotchas
- `quotes_count` had NO trigger until migration 000008 (`maintain_quotes_count`).
- `user_profiles.updated_at` was never set on update until the fix in `user_store.go`.
- Parent-chain / descendants SQL used to return fewer columns than the `Scan` expected
  (would panic at runtime with rows present) — fixed by selecting the count columns too.
- Postgres `INET` columns reject `host:port` strings — strip the port (see `stripPort`
  in `post_handler.go` and `strings.Split(RemoteAddr, ":")[0]` in `auth_service.go`).
- Always use `apperrors.Is(err, code)` to test AppError codes. Comparing
  `err != apperrors.XError(...)` compares pointers and is always true (real bug fixed).

## Cursor pagination
- Keyset pagination on `(created_at, id)` with `ORDER BY created_at DESC, post_id DESC`.
- Cursor timestamps must use `time.RFC3339Nano` — second precision drops posts created
  in the same second (see `post_store.go`).

## Tooling / environment (Docker)
- Full stack runs via root `compose.yaml`; `make dev` = `docker compose up --build -d`.
  Frontend served by nginx (host `:5173`) which reverse-proxies `/api/*` + `/swagger/*`
  to the `api` container — the app is single-origin, so no CORS.
- No Go/Node on the host. Go tooling runs in the `tools` compose service
  (`server` mounted at `/src`); frontend tooling in `web-tools` (`web` at `/src`).
  Both are `profile: tools`, not started by `make dev`.
- `migrate` golang-migrate CLI is built into the api image and applied on start via
  `/app/migrate` (entrypoint `scripts/docker-entrypoint.sh`, POSTGRES_URL env). Manual:
  `make migrate-up` / `make new-migration <name>` (via tools container).
- **Swag resolves `models.X` from the imports of the file containing the annotation** —
  if a handler's annotations reference `models.Foo`, that file must import `models`
  (settings_handler has a `var _ models.UserSettings` for this). Regen: `make swag`
  (writes to `server/docs`).
- Tests: `make test` = `docker compose --profile tools run --rm tools go test ./...`.
  Uses a throwaway DB `social_test` (see `internal/testutil`), create/drop against
  the `db` container via `TEST_DB_ADDRESS=db:5432` (configured in compose).
- Seed: `make seed` runs `/app/seed` in the api image; idempotent (exits early if
  `alice@example.com` exists).
- Docker on NixOS must be enabled (`virtualisation.docker.enable = true`) in
  `~/nixos-config`. Kernel has no legacy iptables + nftables is on; if published
  ports fail, switch Docker to the nftables driver (see comment in configuration.nix).
## Frontend
- `npm run build` needs esbuild/tailwind oxide install scripts approved
  (`npm approve-scripts esbuild @tailwindcss/oxide`); `web/Dockerfile` uses `npm ci`
  which honors package.json `allowScripts`. react-day-picker must be v9 for
  React 19 (was pinned to v8 and broke `npm install` with ERESOLVE).
- Settings PATCH merges partial nested JSON; the payload type must be
  `DeepPartial<UserSettings>` (not `Partial`, which would require nested objects whole).
- `ui/calendar.tsx` was written for react-day-picker v8 (`IconLeft/IconRight`) but v9 is
  installed — use `Chevron`/`PreviousMonthButton`/`NextMonthButton`.
- `ui/sonner.tsx` imported `useTheme` from `next-themes`, which crashes at runtime; the
  app uses the custom `ThemeContext`. Kept on custom provider.
- eslint `@typescript-eslint/no-unused-vars` does NOT ignore `_`-prefixed params — remove
  unused params / use bare `catch {}`.
- `getMediaUrl()` must return `undefined` for empty uuid (backend omits
  `profile_picture_uuid` when none).
- `POST /auth/refresh-token` is a POST with no body (reads cookie) — used in both the
  AuthContext bootstrap AND the 401 retry interceptor.
- Frontend API base URL is relative (`/api/v1`, `VITE_API_BASE_URL`); `npm run dev`
  still works locally thanks to the vite dev proxy to `localhost:2021` (vite.config.ts).
- Frontend build/lint (Docker): `docker compose --profile tools run --rm web-tools npm run build`.
- Phase 1 notifications: migration `000009` creates persistent notifications; migration
  `000010` changes post references to `ON DELETE SET NULL` so history survives deletion.
  `/api/v1/stream` is cookie-authenticated SSE; nginx disables buffering for that route.
  The in-process realtime hub emits `notification.new`, `feed.post_created`, and
  `stream.resync`; frontend EventSource invalidates the relevant React Query caches.
- Phase 2 search: migration `000011` creates `hashtags`/`post_hashtags` and a GIN
  full-text index. Hashtags are normalized lowercase at write time; post search,
  user search, hashtag feeds, and 24-hour top-level-post trends are protected routes.
- Phase 3 post power: migration `000012` adds `posts.edited_at`/`is_pinned`,
  `post_edits`, `polls`/`poll_options`/`poll_votes`, and a partial unique index
  `posts_one_pinned_per_author_idx (author_id) WHERE is_pinned AND NOT soft_deleted`
  (one pin per author). Polls are top-level-only, 2-4 options, one vote/user.
- **Poll votes on a duplicate return 409 via `23505` — the app driver is `pgx/v5`, so
  the unique-violation check must use `*pgconn.PgError` (`github.com/jackc/pgx/v5/pgconn`),
  NOT `*pq.Error` from `lib/pq`** (real bug found by review test). `pq.Array()` still
  works as a `driver.Valuer` for `ANY($1)` params even under pgx.
- **Polls are hydrated in every feed path** via batch `store.Polls.GetForPosts(ids)` +
  service `hydratePolls`, called alongside `hydrateEngagement`. Feeds dropped polls
  before this (critical review finding). `GetFullPostByID`, `GetPinned`, search feed all
  use the same batch helper now.
- Poll/option IDs are global SERIALs (NOT per-post), so `option_id` values are sparse
  across posts — never hardcode `1`; use the ids returned by the API (frontend does).
- `GetByID` (used by `SetPostContextMiddleware`) does NOT filter `soft_deleted`; the
  middleware itself returns 404 for soft-deleted posts so ALL `{postID}` sub-routes
  (edits, poll votes, likes) correctly 404 after deletion — don't add a filter to
  `GetByID` itself, some internals rely on reading deleted rows.
- Swagger: new handler annotations need `@Router` + matching `@Security ApiKeyAuth`
  or the endpoint won't appear in `server/docs` after `make swag`.
- Edit flow: `PostService.Update` is a no-op when content unchanged (no history row);
  `Store.Posts.Update` guards `soft_deleted=FALSE`, `PostService` enforces ownership.
