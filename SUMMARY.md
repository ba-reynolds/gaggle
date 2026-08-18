# SUMMARY — refresh-token-rotation

Refresh tokens now rotate on every use, sessions are grouped into families,
and replayed (theft) tokens kill the whole family. Daily-active users are no
longer logged out on a fixed 15-day-from-login schedule; sessions instead end
by (a) idle timeout (unchanged `JWT_REFRESH_TOKEN_EXPIRATION_TIME`, now
interpreted as the gap between refreshes), (b) explicit logout, or
(c) a detected replay.

## What was changed and why

Review feedback flagged three problems with the old single-long-lived refresh
token: a hard 15-day wall even for daily users, silent logouts, and no way to
detect theft. Full rotation decouples "how long can a session live while the
user keeps using it" from "how long can an idle/stolen token stay valid".

- **Rotation on every refresh** (`AuthService.RefreshToken`): validates the
  presented refresh JWT, then in one transaction (`DB.BeginTx` + rollback on
  error) inserts the successor token and marks the presented one
  `revoked, revoked_reason='rotated'`. The handler now hands the successor
  back through the same httpOnly `refresh_token` cookie
  (`auth_handler.go`) — previously the refresh endpoint returned only the
  access token and never updated the cookie, which would have made rotation
  impossible (clients would keep replaying the revoked token and trip their
  own theft detector). This cookie wiring is the single most important line.
- **Session families** (`refresh_tokens.session_id`): every login/register
  mints a UUID session id; all tokens from one login share it. Migration
  `000016` backfills existing rows with `session_id = refresh_token_id` so
  no one is logged out by the deploy.
- **Theft detection**: a refresh arriving at a token already marked
  `'rotated'` revokes the entire `session_id` family (`reason='theft'`) and
  returns 401. Logout revokes the family too (`reason='logout'`) and is now
  idempotent — logging out with a stale/garbage cookie returns 200 instead
  of 404.
- **SESSION_EXPIRED error code** (`apperrors.SessionExpiredError`): expired
  or revoked refresh tokens map to 401 with `code: "SESSION_EXPIRED"`
  (detected via `errors.Is(err, jwt.ErrTokenExpired)`), distinct from a
  generic `UNAUTHORIZED`. The frontend `AuthContext` toasts "Your session
  has expired" on this code instead of silently dropping the user to the
  login screen.
- **Realtime streams survive rotation**: `GetUserIDFromRefreshToken` (used by
  the SSE stream heartbeat) now checks the session *family* is alive
  (`SessionHasActiveToken`) rather than the exact token, so the stream does
  not drop ~15s after every access-token refresh.
- **Bug found while testing**: JWTs were deterministic — claims use
  second-resolution `iat`/`exp`, so two refresh tokens issued within the same
  second (rotation chains, parallel tabs) were byte-identical and broke the
  entire scheme. Fixed by adding a random `jti` claim in `auth/jwt.go`.
- **Free moderation win**: auth middleware already loads the user per request
  (`GET /users/me` style), so it now rejects soft-deleted accounts on the
  next request instead of leaving them authorized for the access-token
  lifetime (`middleware/token.go`).

## Behavior summary

- Old: logged out exactly 15 days after login no matter what.
- New: a user who refreshes at least once per 15 days stays logged in
  indefinitely; an idle session dies after 15 days without a refresh; a
  logged-out or replayed session dies immediately (family revoked).

## Files touched

- `server/cmd/migrate/migrations/000016_add-refresh-token-session.{up,down}.sql` (new; additive + backfill)
- `server/internal/models/auth.go` — `SessionID`, `RevokedReason` + constants
- `server/internal/store/auth_store.go` — `RotateRefreshToken`, `RevokeSession`, `SessionHasActiveToken`; `CreateRefreshToken`/`GetRefreshToken` read/write new columns
- `server/internal/store/store.go` — Auth interface updated (dropped `MarkRefreshTokenAsRevoked`)
- `server/internal/service/auth_service.go` — rotation, theft handling, family logout, session-aware stream auth
- `server/internal/service/service.go` — `RefreshToken` interface signature
- `server/internal/handlers/auth_handler.go` — refresh now sets the rotated cookie
- `server/internal/middleware/token.go` — reject soft-deleted users
- `server/internal/apperrors/errors.go` — `SESSION_EXPIRED`
- `server/internal/auth/jwt.go` — `jti` claim (unique tokens)
- `web/src/contexts/AuthContext.tsx` — session-expired toast (bootstrap + interceptor)
- `server/internal/testutil/testutil.go` — cookie support + exposed `Service` (test infra)
- `server/internal/handlers/integration_test.go` — 4 new tests

## Verification

- `go build ./...`, `go vet ./...` clean.
- `go test ./...` all pass, including new:
  - `TestRefreshTokenRotation` — rotates every refresh; reusing a rotated
    token → 401 `SESSION_EXPIRED` and the whole family dies (newest token
    included).
  - `TestRefreshTokenRotationChain` — multiple refreshes keep working and
    stream-style auth (`GetUserIDFromRefreshToken`) accepts an older rotated
    token of a live session.
  - `TestRefreshTokenLogoutRevokesFamily` — logout revokes the family; stale
    and garbage-cookie logouts are idempotent 200s.
  - `TestRefreshTokenExpiredRejected` — hand-signed expired refresh token →
    401 `SESSION_EXPIRED`.
- `npm run build` (tsc + vite) passes; `npm run lint` reports the same 14
  pre-existing warnings as the base branch, zero new.

## Things a reviewer should double-check

- **Cross-tab race (known limitation):** two browser tabs refreshing the
  *same* token concurrently — the loser sees `'rotated'` reuse and revokes
  the whole family, logging the user out. The frontend single-flights per
  tab only. An earlier plan discussed a grace window before nuking the family;
  it was deliberately left out for simplicity. If false-positive logouts
  appear in practice, add a short grace period in `AuthService.RefreshToken`
  before calling `RevokeSession`.
- **Migration order**: `000016` must run before deploy. Fine via the
  container auto-migrate; nothing breaks if run late since backfill protects
  existing rows.
- **`t.TempDir()`/test DB**: `test-testutil` drops/recreates `social_test`
  each run; no stored dev data was touched (handler tests build their own).
- Old refresh tokens (issued before this change) keep working: they get
  migrated into single-token families and are legitimately rotated on next
  refresh.

---

# SUMMARY — ui-responsiveness-fixes

Batch of UI responsiveness bugs, fixed across the Go API and the React client.

## What was changed and why

**1. "Replying to @xyz" indicator on reply posts (incl. deleted parents)**
- Server: `models.FullPost` now carries an optional `parent` summary
  (`PostParentInfo`: id + author, `deleted` flag). `postStore.GetParentInfo` fills
  it with one batched query; `service.hydrateParents` is wired into every
  feed/post builder (post, pinned, ancestors, descendants, home/user/bookmarked/
  liked/quotes/list/search feeds).
- Server: `postStore.GetParentChain` now skips soft-deleted chain rows. Previously
  a deleted parent made `GetFullPostByID` fail and 500'd the ancestors fetch, so a
  reply whose parent was deleted rendered "Post not found".
- Frontend: `FeedPost` renders "Replying to @username" (links to the parent post)
  for replies, or "Replying to a deleted post" when `parent.deleted`.

**2. Bookmarks page didn't reflect bookmark tag/status changes**
- Bookmark/unbookmark moved off the generic engagement mutation. They now
  (a) optimistically drop the unbookmarked post from all `['bookmarked', ...]`
  caches, and (b) invalidate `['bookmark-categories']` so the filter badge
  `post_count`s refresh. Bookmark-count optimistic deltas no longer inflate when
  re-categorizing an already-bookmarked post.

**3. Unpinning a post didn't update the profile UI**
- Root cause: React Query retains stale data when a refetch 404s, so the
  `['pinned-post', username]` query kept the old post after unpin. `usePinPost`
  and `useDeletePost` now `removeQueries(['pinned-post', username])` on success.

**4. Edit-history toggle shown even when a post had no edits**
- The "View/Hide edit history" button is now gated on `post.edited_at != null`.
  Only posts with a recorded edit show the toggle and the "edited" marker.

**5. Single-post page had no top bar**
- `PostPage` now has a sticky header (back arrow + "Post" title), mirroring the
  notifications header, so the post isn't jammed at the very top.

**6. Pinned post sat above the tabs and was left-aligned**
- The pinned-post block moved from above the `<Tabs>` into the "Posts" tab
  content (below the tab bar) and is centered via the tab content's
  `items-center` flex (FeedPost is `max-w-xl`).

**7. Notifications "Mark all read" button**
- The button is now hidden when there are no unread notifications. Clicking it
  optimistically marks the cached notifications read (unread rows/dots clear
  instantly) before the server round-trip + invalidation.

**8. "GopherSocial" brand text overflowed its sidebar column (~768–1220px)**
- The sidebar grid column and brand row got `min-w-0`, and the brand span
  `flex-1 min-w-0 truncate`, so it ellipsizes instead of spilling into the main
  column.

## Files touched
- `server/internal/models/post.go` — `PostParentInfo`, `FullPost.Parent`
- `server/internal/store/store.go` — `Posts.GetParentInfo` on the interface
- `server/internal/store/post_store.go` — `GetParentInfo`, soft-delete skip in `GetParentChain`
- `server/internal/service/post_service.go` — `hydrateParents` + 9 call sites
- `server/internal/service/list_service.go`, `search_service.go` — hydrate parents
- `web/src/types/api.ts` — `PostParent` + `Post.parent`
- `web/src/components/FeedPost.tsx` — replying indicator, edit-history gate
- `web/src/hooks/usePost.ts` — bookmark mutation cache handling, pinned-post purge
- `web/src/pages/ProfilePage.tsx` — pinned post moved into Posts tab (centered)
- `web/src/pages/PostPage.tsx` — sticky header
- `web/src/pages/NotificationsPage.tsx` — hide mark-all-read when nothing unread
- `web/src/contexts/NotificationsContext.tsx` — optimistic mark-all-read
- `web/src/layout/SocialMediaLayout.tsx` — sidebar brand truncation

## Verification
- Backend: `go build ./...`, `go vet ./...`, `go test ./internal/handlers/...` pass.
- Frontend: `npm run build` (tsc + vite) and `npm run lint` pass (0 errors).
- Browser (Playwright against the rebuilt stack): all 8 bugs reproduced pre-fix,
  then confirmed fixed post-fix — reply indicator + link, deleted-parent reply
  shows "Replying to a deleted post" (was "Post not found" / 500), post-page
  header, pinned-below-tabs + centered, unpin removes the pinned section,
  edit-history toggle only on edited posts, no brand overflow at 620–1440px,
  mark-all-read clears + hides itself, bookmarks page + badge counts update
  immediately.

## Things a reviewer should double-check
- The running Docker stack may currently be serving another branch: two agent
  sessions share one compose project, and the last `docker compose up --build`
  wins. If `:5173`/`:2021` don't show these fixes, rebuild
  `api` + `web` from this branch.
- `hydrateParents` adds one small query per feed page (batched over the page's
  post ids). Fine at current scales, but worth keeping an eye on with large pages.
- The reply indicator intentionally shows for every post with `parent_id != null`
  in any feed (home, profile, search, bookmarks, ...). Confirm that's the desired
  scope and not too noisy.
- Test data on the shared dev DB was churned by the verification runs (post 39 was
  edited, extra replies/bookmarks/notifications were created, post 40 was pinned/
  unpinned/bookmarked/soft-deleted-then-restored).
