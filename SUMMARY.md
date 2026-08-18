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
