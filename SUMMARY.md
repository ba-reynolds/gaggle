# SUMMARY — google-oauth-analysis

Analysis only (no code changes): how much trouble to add Google OAuth so users
can register/login with it.

## Verdict

**Low-to-medium effort — roughly 3/10 on the trouble scale.** The codebase is
already well set up for this: complete JWT access/refresh token machinery,
a refresh-token cookie, a `users.email` unique index, and a clean service/
handler/store layering. OAuth plugs straight into the existing
`AuthService.CreateRefreshToken` + `GenerateAccessToken` flow. The real work is
OAuth-specific edge cases (username generation, account linking, CSRF state,
testing), not fighting this repo.

Estimate: ~1 focused dev day for a minimal login-only flow; ~2–4 days for a
production-grade version (schema migration, account linking, collision
handling, tests, avatar). Broken down below by concrete repo touchpoints.

## What OAuth actually requires (mapped to this codebase)

### 1. Schema migration — MUST (medium)
- `users.password` is `NOT NULL` (`server/cmd/migrate/migrations/000001_create-users.up.sql:6`),
  but OAuth users have no password. Need migration #16:
  `ALTER TABLE users ALTER COLUMN password DROP NOT NULL` (cleanest) or a
  sentinel password. Notably, `store.userStore.Create` inserts a
  `user_profiles` row too — still works, profile defaults are empty strings.
- Add `google_id TEXT` (the `sub` claim) with a unique index, and treat it
  as the identity key. Falls back to lookup-by-email only if you skip this;
  storing `google_id` survives Google account/email recovery swaps.
- The existing case-insensitive unique index on email
  (`unique_email_case_insensitive`) is free leverage for matching existing
  users.

### 2. Config — trivial
- Add `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URI`,
  `PUBLIC_BASE_URL` to `server/pkg/config/config.go`, `server/.env.example`,
  and `compose.yaml:53`. 2–3 lines each.

### 3. Dependencies — trivial
- `golang.org/x/oauth2` for the code exchange (endpoint
  `google.Endpoint`). For ID-token verification you can reuse the already-
  vendored `github.com/golang-jwt/jwt/v5` to verify Google's RS256 token
  against Google's JWKS, avoiding a new oidc dependency. Verify: issuer
  `accounts.google.com`, `aud` == client id, `email_verified`, signature.

### 4. Backend endpoints — medium, but slots in cleanly
- New handler methods on `AuthHandler` (or a `GoogleOAuthHandler`) mounted in
  `server/internal/api/router.go:63` under `/api/v1/auth`:
  - `GET /auth/google` — build consent URL with a random `state` (stored in a
    short-lived httpOnly cookie, SameSite=Lax, for CSRF), 302 to Google.
  - `GET /auth/google/callback` — validate state cookie, exchange code, verify
    ID token, then:
    - existing user by `google_id` → issue session;
    - existing user by email → **account-link decision** (see #6);
    - new user → derive username from email-prefix/given_name, sanitize to
      `^[a-zA-Z0-9_]+$`, ≤16 chars (`models.RegisterRequest` rules), retry with
      a numeric suffix on `unique_username` collision.
  - On success: reuse `AuthService.CreateRefreshToken` +
    `GenerateAccessToken` and `setRefreshTokenCookie`, then 302 back to the SPA
    root. **Zero new session logic.**
- Resulting session behaves like a normal login, so everything downstream
  (auth middleware `server/internal/middleware/token.go`, refresh, logout) works
  unchanged.

### 5. Frontend — easy
- "Continue with Google" buttons on `web/src/pages/LoginPage.tsx` and
  `SignupPage.tsx` as a plain `<a href="/api/v1/auth/google">`. No fetch/axios.
- The redirect loop lands back at `/`; the existing `AuthContext` bootstrap
  (`refresh-token` cookie → `/users/me`, `web/src/contexts/AuthContext.tsx:67`)
  restores the session automatically. Same-origin via nginx
  (`web/nginx.conf:15`) or the vite dev proxy (`web/vite.config.ts:19`) — no CORS.

### 6. The genuinely fiddly bits
- **Account linking:** a user who signed up with password+same email then logs
  in with Google — merge into the existing account, or reject? Product decision;
  either choice adds code + test surface.
- **Google Cloud Console setup** (manual, unautomatable): create OAuth client,
  authorize redirect URIs that must match the deployed URL exactly
  (`http://localhost:5173/...` dev vs `https://...` prod). Real friction, not a
  code problem.
- **Username generation:** unique-lower index will reject derived handles; need
  a bounded retry/append-counter loop.
- **`email_verified=false`** policy (reject or allow).
- **Testing:** the code exchange hits Google and can't run in a normal test.
  Best approach: hide "exchange code → verified identity" behind a small
  interface and unit-test the callback logic with a fake — consistent with the
  existing `server/internal/handlers/integration_test.go` style. Moderate.
- **Avatar:** pulling the Google avatar into the existing media/profile-picture
  system is extra work (download + store via `MediaService`); easiest to skip for
  v1 and leave the avatar unset.

## Files that would be touched for a real implementation
- `server/cmd/migrate/migrations/000016_*.{up,down}.sql` (new migration)
- `server/internal/models/auth.go` (OAuth callback/state models)
- `server/internal/store/user_store.go` (+`GetByGoogleID`, nullable-password `Create`)
- `server/internal/service/auth_service.go` (+`LoginWithGoogle`)
- `server/internal/handlers/auth_handler.go`, `server/internal/api/router.go`
- `server/pkg/config/config.go`, `server/.env.example`, `compose.yaml`
- `web/src/pages/LoginPage.tsx`, `web/src/pages/SignupPage.tsx`

## Reviewer checkpoints
- No code was changed in this branch — it is analysis-only.
- Confirm the account-linking policy (merge vs reject) before starting, as it
  dominates the design.
- During dev, the Google redirect URI must exactly match what nginx/vite are
  listening on.

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
