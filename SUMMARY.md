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
  cards used everywhere in the app**, with a **C-shaped gutter connector**
  joining each adjacent pair (parent -> child):
  - a thin vertical line runs in the avatar gutter (behind the avatars), and
    each child shows a short right-pointing elbow where the line turns in
    toward its profile picture;
  - cards keep their normal spacing (`mb-2`) and untouched styling;
  - the connector is drawn via an optional `thread: 'first' | 'middle' |
    'last'` prop on `FeedPost` (overlay only — non-thread usage is identical);
  - `PostPage.tsx` maps `threadPosts = [...parentChain, post]` and assigns
    positions (`first` topmost, `last` = current post, `middle` in between).
- Earlier experimental approaches (a gutter rail on FeedPost, a dedicated
  `ThreadPanel`, then simple `Separator` dividers between cards) were all
  removed in favor of this elbow connector.

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
