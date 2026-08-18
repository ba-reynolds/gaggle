# SUMMARY — post-thread-and-bookmark-fixes

Verified a batch of five reported UI bugs against the running app and fixed the
two that were still broken. The other three turned out to be already fixed on
`main` (from the earlier `ui-responsiveness-fixes` merge) and are confirmed
working end-to-end in the browser.

## What was changed and why

**1. Reply/child ordering on a single post page (was broken → fixed)**
- Root cause: `postStore.GetDescendants` ordered direct replies
  `ORDER BY created_at ASC`, so when you replied to a post your new reply was
  appended at the *bottom* of the replies list.
- Fix: `server/internal/store/post_store.go` GetDescendants now orders
  `created_at DESC` for both the initial and cursor-paged queries, and the paged
  query uses `created_at < $cursor` (older pages) instead of `>`. New replies now
  land at the top, driven by the existing `['post', parentId]` invalidation on
  post-create.

**2. Single-post page thread layout + visual connection (was broken → fixed)**
- Root cause: `PostPage` rendered the current post first and the parent chain
  below it (reply on top of the post being replied to), with no visual
  connection between them.
- Fix (final, after several design iterations from review feedback): the chain
  (ancestors furthest-first + the current post) renders as the **same FeedPost
  cards used everywhere in the app**, untouched. The connector lives entirely
  in a **gutter to the left of the whole chain** (`PostPage.tsx`): each row is a
  `flex` of `[gutter | card]`, so each gutter cell stretches to its card's
  height and the cells stack flush. Inside the gutter:
  - a thin vertical rail runs the full thread height, and each child row shows
    a short right-pointing elbow (C-shape) horizontally aligned with that
    post's profile picture (the tick sits at the avatar's vertical center and
    points toward the card);
  - `first` starts the rail level with the parent's avatar, `last` ends it at
    the current post's avatar;
  - the card's normal `mb-2` spacing is preserved while the rail stays
    continuous (gutter cells are flush).
- `FeedPost` is back to its original, untouched form — no connector prop, no
  overlay, so other pages using it are pixel-identical.
- Final connector style (after several review iterations): a 2px vertical rail +
  a straight 2px horizontal tick at every post's avatar level, joined by an
  **empty (border-only) circle** that acts as the node at each post.
  - The first post's line starts **at** its circle (no line above the
    horizontal), the rail runs through middle posts (entering and leaving each
    circle), and the last post's line stops at its circle.
  - All segments are straight divs (no border-drawn curves); the tick has
    `rounded-full` caps and leaves the circle's right edge toward the post's
    profile picture.
- Earlier experimental approaches (gutter rail on FeedPost, a dedicated
  `ThreadPanel`, `Separator` dividers, an in-card elbow overlay) were all
  removed in favor of this left-gutter connector.

**3. Bookmarks category counter (already fixed on main → verified)**
- Re-categorizing a bookmarked post through the bookmark popover calls
  `useBookmarkPost`, whose `onSuccess` invalidates `['bookmark-categories']`;
  the badge `post_count`s refetch. Confirmed in the browser: moving the only
  bookmarked post Travel→General updated both badges (Travel 1→0, General 0→1)
  and back.

**4. Pinned post position on profiles (already fixed on main → verified)**
- The pinned block sits inside the "Posts" tab content (below the tab bar) and
  is centered. Confirmed by geometry: pinned block center == tabs center
  (x=587) and pinnedY (617) > tabs bottom (588).

**5. "Replying to @user" text (already fixed on main → verified)**
- `FeedPost` renders the "Replying to @username" line (or "Replying to a deleted
  post") for `parent_id != null` posts; the server hydrates the `parent` summary
  on every path including the single-post detail. Confirmed on `/post/41`.

## Verification
- Backend: `go test ./...` passes (tools container).
- Frontend: `npm run build` (tsc + vite) and `npm run lint` pass (0 errors).
- Browser (playwright + headless chrome, logged in as alice):
  - `/bookmarks` re-categorize → badge counts update. PASS
  - `/profile/alice` pinned post below tabs + centered. PASS
  - `/post/41` parent above current post, 2 connector segments, "Replying to
    @alice" text. PASS
  - reply to `/post/40` → new reply is the first reply. PASS

## Double-check for reviewers
- The connector-line visuals should be eyeballed (screenshots under
  `/tmp/opencode/verify/*.png` from the session). I could not view images, so
  centering/line geometry was asserted numerically instead.
- `GetDescendants` ordering flipped to DESC: confirm no other consumer assumed
  ASC (only the single-post endpoint uses it; the integration test
  `TestPostWithAncestorsAndDescendants` passes).
- The DB holds test data created during verification (a reply set on post 40,
  and post 40 pinned on alice's profile). Harmless dev data.
- Out of scope, observed while testing: `POST /auth/refresh-token` returns 500
  when no cookie is present (`auth_handler.go` maps `http.ErrNoCookie` to
  InternalServerError). The app swallows it during boot, but it's a widespread
  500 in logs. Consider returning 401 in a follow-up.

---

# SUMMARY — pinned-post-menu-fixes

Pinning a post from the main timeline never showed the "Unpin from profile"
option on the newly pinned post. Root cause was a server-side cache, not the
client local store.

## What was changed and why

**Root cause:** `GET /posts/feed` is served from a 60-second Redis cache
(`feed:home:{userID}:{cursor}`) in `handlers.GetHomeFeed`. When a user pinned a
second post, the DB and the `/users/{username}/pinned` endpoint updated
correctly, but `PinPost` never invalidated the home-feed cache — so the main
timeline kept serving the previous `is_pinned` flags for up to 60s. The
frontend correctly refetched `['feed']` after the pin, but the API returned the
stale cached copy. Result: the newly pinned post's menu still showed "Pin to
profile" (no "Unpin from profile" option).

Reproduced against the live stack: after pinning post 39, `/posts/feed`
returned 40 pinned / 39 not (stale), while the DB and pinned endpoint returned
39 pinned / 40 not (truth).

**Fix:** `PostHandler.PinPost`, `UnpinPost`, `UpdatePost`, and
`DeletePostByID` now all call `invalidateFeedForUserAndFollowers(ctx, user.ID)`
after a successful write — the same helper `CreatePost` and the engagement
handler already use. The feed must be dropped on any write that changes
`is_pinned` (pin/unpin), post content/`edited_at` (edit), or post
existence (delete), because that data is embedded in every cached home-feed
payload for the author *and* their followers.

## Files touched
- `server/internal/handlers/post_handler.go` — cache invalidation added to
  `PinPost`, `UnpinPost`, `UpdatePost`, `DeletePostByID` (nil-safe helper,
  no-op when Redis is absent)
- `.opencode/project-notes.md` — recorded the Redis home-feed cache gotcha

## Verification
- `go build ./...` and `go vet ./...` pass.
- `go test ./...` passes (handlers integration tests cover pin/unpin/update/
  delete flows; the harness runs without Redis).
- Rebuilt the `api` container from this branch and repeated the repro sequence:
  seed cache → pin 40 → home feed immediately shows 40 pinned / 39 unpinned
  (was stale before); restored pin 39 → feed correct. DB state left as found
  (post 39 pinned).

## Things a reviewer should double-check
- `UpdatePost` now invalidates the feed for the author + followers because a
  content/`edited_at` change shows up in cached home-feed payloads; this closes
  the same staleness class (edited posts showing old content in the timeline).
- Redis staleness is not covered by `go test` (test harness passes `nil` rdb,
  no seam to fake the concrete `*cache.Client`). Rebuilding/verifying against
  the live stack is the regression check. A Redis-backed test would need a
  cache interface or a test Redis.
- The running API container now serves this branch's build (shared single
  compose stack) — other parallel agent sessions may see their branch lose the
  running build.

---

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

# SUMMARY — fix-profile-tabs-and-user-relations

Profile tabs, follow lists, and user-management (block/mute) for the profile page.

## What was changed and why

**1. Replies & Media profile tabs now work**
- The tabs existed but always rendered "No replies yet" / "No media yet" — there
  was no backend feed for them and the frontend never fetched anything.
- Server: `postStore.runUserFeedQuery`/`buildUserFeedQuery` centralize the
  user-feed SQL with three modes: top-level ("all"), replies-only
  (`parent_id IS NOT NULL`), and media (`EXISTS (SELECT 1 FROM post_media …)`).
  New endpoints `GET /users/{username}/replies` and `GET /users/{username}/media`
  (handlers `GetUserRepliesFeed`/`GetUserMediaFeed`, service
  `GetUserRepliesFeed`/`GetUserMediaFeed`), keyset-paginated like the posts feed
  and hydrated with media/engagement/polls/parents.
- Frontend: `getUserReplies`/`getUserMedia` + `useGetUserReplies`/`useGetUserMedia`
  infinite queries. `ProfilePage` wires both tabs through a new `ProfileFeedTab`
  component (infinite scroll, loading + empty states). The shared
  `POST_QUERY_KEYS`/`invalidatePostQueries`/`invalidateBookmarkQueries` and
  `useCreatePost` now invalidate `user-replies`/`user-media` so engagement
  toggles and new posts refresh those tabs.

**2. Following / Followers view**
- API + hooks for followers/following already existed but no page used them, and
  the backend response shape didn't match the frontend type: it returned
  `{followers: [UserWithProfile…]}` while `fetchUserFollowers` typed
  `{items: UserProfileResponse[]…}`.
- Server: `UserFollowersResponse`/`UserFollowingResponse` now return flat
  `items: UserProfileResponse[]` (converted via `ToProfileResponse`), matching
  the app-wide "paginated feeds use `items`" convention. `GetFollowers`/
  `GetFollowing` hydrate viewer-relative relationship flags (see #3).
- Frontend: new `FollowListPage` (`/profile/:username/followers` and
  `/profile/:username/following` routes) lists users with avatar/name/bio and
  Follow/Unfollow buttons + Load-more pagination. `ProfilePage` Following/
  Followers counts are now links to those routes.

**3. Profile user-management menu (three dots: follow, mute, block)**
- `UserProfileResponse` gained viewer-relative `is_following` / `is_blocked` /
  `is_muted`. `GetUserProfileByUsername` (and `GetMe`) hydrate them via
  `handler.hydrateProfileRelationship`; `GetFollowers`/`GetFollowing` hydrate
  them via the new batched `store.GetRelationshipStatuses`.
- Server mute support: migration `000016` adds `'mute'` to the
  `user_relationships.relationship_type` CHECK. `GetRelationshipStatus` now reads
  ALL rows for a pair (follow+mute coexist), returns `is_muted`. New
  `POST/DELETE /users/{username}/mute` handlers (`MuteUser`/`UnmuteUser`);
  `CreateRelationship` gained a mute branch (idempotent) and follow is now
  type-safe (`Exists` instead of a single-row read that previously would have
  clobbered a coexisting mute row). Unfollow/unblock/unmute delete only their own
  relationship type (`DeleteByType`) — previously unfollow/unblock deleted every
  row between the pair.
- Mute is meaningful: `notification_service.Create` now drops notifications whose
  actor the recipient has muted (suppresses replies/likes/reposts/follows/mentions
  without hiding content).
- Frontend: `muteUser`/`unmuteUser` + `useMuteUser`/`useUnmuteUser`. `ProfilePage`
  header (non-self) now has a Follow button, Message button, and a `MoreHorizontal`
  DropdownMenu with Mute/Unmute and Block/Unblock. All relationship mutations
  (`useFollowUser`/`useUnblockUser`/new mute hooks) invalidate
  `profile`+`user-followers`+`user-following` via a shared helper.

## Files touched
- `server/cmd/migrate/migrations/000016_add-mute-relationship.{up,down}.sql` — new
- `server/internal/models/user_relationship.go` — `RelationshipStatus.IsMuted`; followers/following → `Items []UserProfileResponse`
- `server/internal/models/user.go` — `UserProfileResponse` relationship flags
- `server/internal/store/store.go` — `UserRelationships` + `Posts` interfaces
- `server/internal/store/user_relationship_store.go` — multi-row status, `GetRelationshipStatuses`, `Exists`, `DeleteByType`, flat followers/following
- `server/internal/store/post_store.go` — `GetUserReplies`, `GetUserMediaFeed`, shared query builder/runner
- `server/internal/service/service.go`, `user_relationship_service.go`, `post_service.go` — mute flow, type-scoped delete, replies/media feeds, statuses passthrough
- `server/internal/service/notification_service.go` — mute suppresses notifications
- `server/internal/handlers/user_relationship_handler.go` — Mute/Unmute + viewer status hydration on lists + unfollow/unblock type-scoped
- `server/internal/handlers/post_handler.go` — replies/media feed handlers
- `server/internal/handlers/user_handler.go` — profile relationship hydration
- `server/internal/api/router.go` — 4 new routes
- `server/docs/*` — regenerated swagger
- `server/internal/handlers/integration_test.go` — new end-to-end test
- `web/src/types/api.ts` — relationship flags
- `web/src/api/user.ts`, `web/src/api/posts.ts` — mute/unmute/replies/media calls
- `web/src/hooks/useUser.ts`, `web/src/hooks/usePost.ts` — new hooks + invalidation scope
- `web/src/pages/ProfilePage.tsx` — replies/media tabs, follow/mute/block menu, counts links
- `web/src/pages/FollowListPage.tsx` — new
- `web/src/App.tsx` — followers/following routes

## Things a reviewer should double-check
- **`GetRelationshipStatus`/`DeleteByType` behavior change**: the old single-row
  `GetRelationshipStatus` + pair-wide `Delete` worked only because follow and block
  were mutually exclusive. With mute coexisting, delete must be type-scoped. Unblock
  no longer deletes a follow row (block already removed it at block time). Block
  still clears ALL rows between the pair in both directions (existing, deliberate).
- **Mute notification suppression** covers notifications created through
  `notification_service.Create` (likes/reposts/replies/follows/quotes/mentions).
  DM unread badges are not muted.
- **`Users with is_following=false by default`**: relationship flags default to
  false on every profile-shaped response; only the profile and followers/following
  endpoints hydrate them. Search/suggested/likers/reposters intentionally still
  return false — do not rely on the flag there without hydrating.
- **Swagger** regenerated with `make swag` — new endpoints (`/users/{u}/mute`,
  `/replies`, `/media`) are present in `docs`.
- **Frontend lists** (`FollowListPage`) use raw API calls + local state rather
  than React Query, so follow/unfollow there only updates the local row; the
  ProfilePage counts refresh on navigation (invalidation is query-key based).

---

# SUMMARY — cloud-deploy-email-analysis

Analysis spike (no code changes): assessed cloud-readiness of the current
stack and researched email-verification alternatives for the domain-buying /
cloud-deployment plan.

## What the analysis found

**Stack is already cloud-friendly.** Two custom Dockerfiles (`server/`,
`web/`), `docker compose` single-origin stack, config 100% env-var driven
(`server/pkg/config/config.go`), static cgo-free Go binaries on `alpine`
(portable across AWS/GCP/etc.), migrations applied on container start
(`scripts/docker-entrypoint.sh`).

**What must change before production deployment** (blockers / risks):
- Production secrets live in repo defaults — compose hardcodes
  `JWT_SECRET=dev-secret-change-me` and `.env.example` ships
  `DB_PASSWORD=teeth`. Must come from a secret manager (e.g. AWS Secrets
  Manager) with NO shipping default.
- `COOKIE_SECURE=false` (compose.yaml:78) — the refresh-token auth cookie
  would be sent over plain HTTP in prod. Needs to be `true` behind TLS and
  `X-Forwarded-Proto` honored by the auth middleware (currently reads
  `RemoteAddr`, see `internal/auth/auth_service.go`).
- `POSTGRES_URL` with `sslmode=disable` and hardcoded
  `postgres://white:teeth@db:5432/social` in compose; cloud-managed DBs
  (RDS/Cloud SQL) need real credentials / SSL.
- Redis has no password (`REDIS_PASSWORD` is wired through config but the
  compose service starts unauthenticated) — fine behind a VPC, bad if
  exposed.
- Media is stored on a local docker volume (`api_media`); deployable as-is
  (`MEDIA_DIR` is config) but a managed object store (S3/Cloud Storage) is the
  right production answer.
- Logs go to a file (`LOGGING_FILENAME=logs/logs.log`) — for cloud log
  aggregation (CloudWatch / GCP Logging) they should go to stdout/stderr.
- `MIGRATE_ON_START=true` runs migrations on every API instance start —
  acceptable at single-instance scale, but should be pulled out into a
  dedicated step (or use the DB job) before scaling horizontally.

**Email verification does NOT exist yet.** No SMTP/mail dependency, no
verification flow — `AuthService.Register` creates the user and issues tokens
immediately (`service/auth_service.go:223`), no signup email is sent, no
`email_verified` column. Emails are only collected+validated for uniqueness.
The `.env.example` even notes `GEMINI_API_KEY` is "currently unused".

**Email options recommended (in order):**
1. AWS SES (+ Route 53) — free tier, full AWS integration, cheap at scale,
   needs domain + Easy DKIM via Route 53. Most work to wire up and manage
   reputation/limits, best fit if they commit to AWS.
2. Resend — best DX, generous free tier (3k/mo), great delivery + analytics,
   React email templates, SDK is trivial. If not married to AWS, this is the
   easiest path.
3. Others (SendGrid / Postmark / Mailgun) — all viable, mostly worse DX or
   pricing tiers.

All ESPs work fine behind the domain they buy. Implementation shape (future
work): add `email_verified_at` + signed-token verification links, then a tiny
`mailer` service interface (SES vs Resend are drop-in behind it).

**Domain recommendation.** Cloudflare for registration + DNS (~$10/yr, at-cost
DNS, free tunnel/TLS, one place for DNS records that SES/Resend need: SPF,
DKIM, route) — unless they want single-vendor simplicity with AWS, in which
case Route 53 (registration enforced from handful of TLDs) + ACM for the
TLS cert. Cheapest real deploy path: `docker compose` services on a small
VPS with Cloudflare Tunnel in front (no EIP/LB needed, still HTTPS).

## Files touched
- None (analysis-only spike; changes left for a follow-up implementation task).

## Verification
- No code changed — nothing to test. Findings are grounded in
  `compose.yaml`, `server/pkg/config/config.go`, `server/scripts/docker-entrypoint.sh`,
  `web/nginx.conf`, `server/internal/service/auth_service.go`, `internal/store/user_store.go`,
  and the `.env.example` files.
- Note: the shared Docker stack was NOT rebuilt; this branch is tied to the
  shared compose project like every other agent-branch worktree.

## Things a reviewer should double-check
- Confirm the interpretation that "add email sending" is a follow-up task and
  not part of this spike.
- If implementing: lean on the `service` layer already used everywhere —
  an `EmailService` behind an interface (SES/Resend impls) matches the
  repo's existing service/store pattern.

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
