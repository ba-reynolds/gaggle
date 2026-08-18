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
