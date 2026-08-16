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

## Tooling / environment (NixOS)
- Dev env via root `flake.nix`; enter with `nix develop`.
- `migrate` CLI is NOT on PATH — run via Makefile which uses `go run -tags=postgres
  github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1`. The nixpkgs `go-migrate`
  binary is broken (gosnowflake CA panic).
- `swag` isn't in nixpkgs — the Makefile `swag` target uses `go run ...swag@latest`.
  **Swag resolves `models.X` from the imports of the file containing the annotation** —
  if a handler's annotations reference `models.Foo`, that file must import `models`
  (settings_handler has a `var _ models.UserSettings` for this).
- Local Postgres/Redis (no Docker): `cd social-back && make dev-services` /
  `make dev-services-stop`. Data lives in `/tmp/gophersocial`.
- Tests: `go test ./...` uses a throwaway DB `social_test` (see `internal/testutil`),
  created/dropped automatically against the local Postgres.
## Frontend
- `npm run build` needs esbuild/tailwind oxide install scripts approved
  (`npm approve-scripts esbuild @tailwindcss/oxide`). react-day-picker must be v9 for
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
- Build/lint: `nix develop --command bash -c 'cd social-front && npm run build'`.
