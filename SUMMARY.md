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
