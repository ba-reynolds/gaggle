# SUMMARY — poll-question-trending

Two changes: (1) the poll's "question" no longer has its own composer field —
the post text box IS the question — and (2) the right-rail Trending box now
actually shows data and its "Show more" button works.

## Poll question lives in the post text box

**What was changed and why**

- `web/src/components/ComposeContent.tsx`: removed the separate "Poll question"
  input and its `pollQuestion` state. The submit payload now sends
  `poll.question = text` (the main text box). The submit button is disabled
  when a poll is enabled but the text box is empty, since the question must
  be written there.
- `server/internal/service/post_service.go`: `PostService.Create` now echoes
  the post content into `poll.question` instead of trusting the request body,
  so the stored question always mirrors the text box regardless of client.
  `validatePoll` dropped the 140-char cap (the post text has no such limit)
  and keeps only the non-empty check (which rejects media-only polls, e.g.
  image + poll + blank text).
- `web/src/components/PollCard.tsx`: removed the question line from the card —
  the post content above the card already renders it — leaving the vote count
  top-right.
- `server/internal/handlers/integration_test.go`: added an assertion that the
  stored poll `question` equals the post content ("pick one", not the "Which?"
  sent in the request body).

## Trending box was empty ("No trends yet")

**Root cause** — not a backend bug: `/trends` was returning an empty list
because the DB had zero hashtag rows. Hashtag syncing only happens in the
**service layer** (`Hashtags.SyncPost` in `post_service.go`), but the seed
script (`server/cmd/seed/main.go`) calls `store.Posts.Create` **directly**,
bypassing the service — so the seeded `#programming #coding` post never
created `hashtags`/`post_hashtags` rows. With no hashtagged posts in the last
24h (the trends window), the API honestly returned `[]` and the sidebar showed
"No trends yet."

**What was changed and why**

- `server/cmd/seed/main.go`: `seedPosts` now calls
  `store.Hashtags.SyncPost(ctx, tx, post.ID, post.Content)` after creating
  each top-level and reply post (inside the same transaction, mirroring the
  service layer). A few seed post texts also gained hashtags (`#sunset #nature`,
  `#art`, `#fitness`, `#food`, `#music`, `#photography`) so a fresh seed
  produces a populated trending box instead of a single hashtag.
- `web/src/layout/SocialMediaLayout.tsx`: the "Show more" button under the
  Trending box was a link-less placeholder that did nothing — it now navigates
  to `/explore?tab=trending`.
- `web/src/pages/ExplorePage.tsx`: the Explore page now reads `?tab=trending`
  from the URL to select the Trending tab (controlled `Tabs`), so deep-linking
  from the sidebar actually lands on the full trends list.

## Files touched

- `web/src/components/ComposeContent.tsx`
- `web/src/components/PollCard.tsx`
- `server/internal/service/post_service.go`
- `server/internal/handlers/integration_test.go`
- `server/cmd/seed/main.go`
- `web/src/layout/SocialMediaLayout.tsx`
- `web/src/pages/ExplorePage.tsx`
- `SUMMARY.md` (this section)

## Things a reviewer should double-check

- **Existing (already-seeded) databases** keep showing "No trends yet" until
  either the DB is wiped + re-seeded, or a user posts with a hashtag. The seed
  is idempotent and exits early when `alice@example.com` exists, so it will
  NOT backfill hashtags into an existing DB. This was left as-is deliberately;
  if instead we want the seed to "touch up" seeded timestamps/hashtags on
  re-run, that's a follow-up.
- **Trends are windowed to the last 24 hours** by design
  (`hashtag_store.go:Trends`). Seeded content ages out after a day; the box
  returning to "No trends yet" on a quiet instance is expected, not a bug.
- **Poll question API**: `polls.question` is still stored and serialized
  (`Poll.question`), now always echoing the post content. Any code that reads
  `poll.question` expecting a distinct value will see the content instead.
  The UI no longer renders it.
- **Poll posts now require text** (submit is disabled / backend rejects empty
  question). Media-only poll posts (image + poll + no text) are no longer
  possible — intended, since the text box is the question.

---

# SUMMARY — move-themes-to-settings

---

# fuzzy-search-results

---

---

# SUMMARY — fix-bookmark-like-increments-view

## Problem

On a post's detail page, clicking **like** or **bookmark** bumped the post's
view count by 1.

### Root cause

`GET /posts/{postID}` records a view on *every* request
(`post_handler.go:252` → `PostEngagementService.AddView` → plain
`INSERT INTO post_views`). The detail page's like/bookmark React Query
mutations invalidate the `['post', postId]` query on success
(`usePost.ts:170` / `usePost.ts:420`), which refetches the same GET endpoint.
Each refetch appended a new `post_views` row and its trigger
(`maintain_views_count`, migration 000007) incremented `posts.views_count`.

So the flow *view page → like → refetch* produced 2 view rows for one human
viewing one post; every further engagement bump re-inserted another row.

## Fix

Make a view idempotent per authenticated user + post, instead of per HTTP
request.

- **Migration `000019_dedupe-post-views`**:
  - Deletes older duplicate `post_views` rows (keeps the newest per
    `(post_id, user_id)` for logged-in users) and decrements the denormalized
    `posts.views_count` by the number of excess rows per post.
  - Adds partial unique index `post_views_user_dedup_idx ON post_views
    (post_id, user_id) WHERE user_id IS NOT NULL`.
- **`AddView`** (`post_engagement_store.go`): now `INSERT ... ON CONFLICT DO
  NOTHING` so a repeat visit from the same user silently no-ops instead of
  erroring on the new index.

### Semantics

`views_count` now means "distinct authenticated viewers" (+ anonymous views,
unchanged), not "page loads". This matches the user expectation that
interacting with a post doesn't inflate its view count, and also fixes the
count for any other repeated fetch (back/forward navigation, refetch on window
focus, etc.).

## Files touched

- `server/cmd/migrate/migrations/000019_dedupe-post-views.up.sql` (new)
- `server/cmd/migrate/migrations/000019_dedupe-post-views.down.sql` (new)
- `server/internal/store/post_engagement_store.go` — idempotent insert
- `server/internal/handlers/integration_test.go` —
  `TestViewsAreDeduplicatedPerUser` (repeat fetch from same user must not bump
  the count; a different user must still count).

## Verification

- `make test` (tools container `go test ./...`): all pass, including the new
  `TestViewsAreDeduplicatedPerUser` and the existing `TestViewsAreRecorded`.

## Reviewer double-checks

- **Migration applies on existing data**: the dedup DELETE + views_count
  correction runs before the unique index is created; verify the count
  correction matches the number of rows actually deleted (it uses the same
  `(post_id, user_id)` grouping).
- **Partial index / NULLs**: anonymous views have `user_id = NULL`, so they are
  excluded from the unique index and keep their per-request behavior. If we
  later want anonymous dedup (per IP/user-agent), that needs a separate partial
  index + conflict target — out of scope here.
- **Old binary + new index race**: until the api container is rebuilt with this
  branch, its `AddView` insert can hit the new unique constraint (a unique
  violation). The handler ignores `AddView` errors, so it only logs — no user
  impact. Rebuild both `api` + `web` from this branch when merging.
- **Migration version**: `000019` is the next free version on this branch;
  confirm no parallel branch picked the same number before merging (see
  project-notes "duplicate migration file" hazard).

---

# SUMMARY — account-and-post-privacy

Post-level visibility and account-level privacy are now actually enforced end
to end. The compose-box visibility toggle was cosmetic (the payload had no
field, no column existed); the settings `profileVisibility` dropdown was
persisted to JSONB but never read. Both are now real.

## What was changed and why

**Post-level visibility** (`posts.visibility`: `public` | `followers` |
`mentions` + `posts.mentioned_user_ids int[]`):
- The compose dropdown ("Everyone" / "Followers only" / new "Only people you
  mention") now round-trips: `CreatePostPayload.visibility` →
  `Post.Visibility` → written at create (`postStore.Create`/`CreateQuotedPost`).
- `resolveVisibilityAndMentions` (PostService.Create/QuotePost) normalizes the
  value (empty → `public`), validates it, and for `mentions` resolves
  `@username`s in the content to user IDs (stored in `mentioned_user_ids`). A
  mentions-only post that mentions nobody is rejected (400).
- One centralized enforcement point, `service.filterVisiblePosts(ctx, st, posts,
  viewerID)` — a package-level (batch) helper like `hydrateHelpers`. For each
  unique author it runs one `GetRelationshipStatuses` + one `Users.GetIsPrivate`,
  then keeps a post when: author == viewer, OR visibility == public, OR
  (followers AND viewer follows author), OR (mentions AND viewer is in
  `mentioned_user_ids`). Called from every feed path (home, user, replies,
  media, bookmarks, likes, quotes, list, search/hashtag), ancestor/descendant
  chains, `GetFullPostByID`, and `GetPinned` (404 when filtered out).
- Engagement writes are gated: like/repost/bookmark/vote/quote call
  `PostService.CanViewPost` first (404 for posts the actor can't read), so a
  stranger can't like or vote on a followers-only post. Quotes also can't be
  created against unviewable posts.
- Feed `HasMore`/cursor is computed from raw rows, so pagination keeps working
  after filtering (a page with hidden posts just returns fewer items).

**Account-level privacy** (`users.is_private`, source of truth; backfilled from
`user_settings.privacy.profileVisibility` in migration `000017`):
- `profileVisibility` is synced to `users.is_private` on every settings PATCH
  (`settings_handler`), and `UserProfileResponse.is_private` exposes it.
- Enforcement lives in `filterVisiblePosts` (private authors only expose posts
  to followers) — a profile *shell* (display name/bio/counters/Follow button)
  stays public, matching the chosen "show shell, hide content" behavior. Feeds
  for strangers return empty; single posts 404. Blocked-by-them viewers keep
  the public shell + can still see plain *public* posts (ghost view); blocks
  already remove the follow relationship so followers/mentions-only content is
  hidden from them automatically.
- `friends` maps to followers-only (this app has no separate "friends" circle).

**Frontend**: ComposeContent sends `visibility` (+ "Only people you mention"
with an `@` icon); FeedPost shows a small `Users`/`AtSign` badge+tooltip for
restricted posts; ProfilePage shows a lock notice on private accounts you don't
follow; `Post.visibility`/`UserProfileResponse.is_private` in the API types.

## Files touched

- `server/cmd/migrate/migrations/000017_account-post-privacy.{up,down}.sql` (new)
- `server/internal/models/post.go` — `Post.Visibility`, `Post.MentionedUserIDs`,
  `CreatePostPayload.Visibility`
- `server/internal/models/user.go` — `User.IsPrivate`, `UserProfileResponse.IsPrivate`
- `server/internal/store/post_store.go` — create/quote writes visibility+mentions;
  `GetPostVisibilities`; `GetFullPostByID` returns them; `scanMentionedIDs`
  adapter + `nonNilIntSlice` (pq int[] scanning/values quirks)
- `server/internal/store/user_store.go` — `SetPrivate`, `GetIsPrivate`; is_private
  in all user scans
- `server/internal/store/user_relationship_store.go` — is_private in
  followers/following scans
- `server/internal/store/store.go` — interfaces (`SetPrivate`, `GetIsPrivate`,
  `GetPostVisibilities`)
- `server/internal/service/post_service.go` — `resolveVisibilityAndMentions`,
  `CanViewPost`, `filterVisiblePosts`, wired into every feed + single-post paths
- `server/internal/service/{list,search}_service.go` — filterVisiblePosts in
  list/hashtag/search feeds
- `server/internal/service/{service,user_service}.go` — `Users.SetPrivate`
- `server/internal/handlers/post_handler.go`, `post_engagement_handler.go` —
  visibility passthrough, `requirePostVisible` gate, quote/likers/reposters gates
- `server/internal/handlers/settings_handler.go` — is_private sync
- `web/src/{components/ComposeContent,components/FeedPost,pages/ProfilePage,hooks/usePost,types/api}.ts`
- `server/internal/handlers/integration_test.go` — `TestPostVisibility`,
  `TestAccountPrivacy`
- `server/docs/*` — swagger regenerated (`make swag`)

## Verification

- `go build ./...`, `go vet ./...` clean; `go test ./...` passes, incl. new
  `TestPostVisibility` (public/followers/mentions access across single-post,
  profile feed, and like gating; invalid-visibility 400s; mentions-with-no-
  mention 400) and `TestAccountPrivacy` (private/friends/public toggles, shell
  stays public, stranger 404s + empty feed, follower access restored).
- `npm run build` (tsc + vite) passes; `npm run lint` = the same 14 pre-existing
  warnings as base, zero new.

## Things a reviewer should double-check

- **Any FUTURE feed/hydration consumer must call `filterVisiblePosts`** or it
  leaks restricted posts — the privacy surface is a service-layer convention,
  not a DB constraint. See `.opencode/project-notes.md`.
- **Media is still public by UUID** (`GET /media/{uuid}`, unguessable-token
  design, `<img>` can't send auth). A followers/mentions-only post's media is
  reachable if the UUID is known. Pre-existing architecture tradeoff; the post
  content and engagement are gated.
- **Pagination after filtering**: `HasMore` is computed from raw feed rows, so a
  denser-than-normal run of hidden posts returns short pages (rare; matches
  Twitter's approach). Cursor progression remains correct.
- **Replies** are filtered per reply author (a reply to a followers-only post by
  a non-follower is hidden from strangers) but there is no "reply visibility
  inheritance" — each post's own visibility rules apply. Account privacy gates
  the whole thread for strangers anyway.
- **`mentioned_user_ids` is resolved at create time** from the exact stored
  content; editing content (POST /posts/{id}) does not re-resolve mentions or
  change visibility. `visibility`/mentions are fixed at creation.
- Home-feed Redis cache is per-viewer and stored AFTER filtering, and
  create/edit/delete already invalidate — no cache invalidation was needed for
  the privacy toggles themselves (changing privacy only affects who already
  could/couldn't see the author's own feed, which is follow-driven).

---

---

---

# SUMMARY — move-themes-to-settings

---

## What was changed

Post search (`GET /search?type=posts`) now uses case-insensitive **substring**
matching instead of Postgres full-text (lexeme) matching.

Previously the store claused on
`to_tsvector('simple', p.content) @@ plainto_tsquery('simple', $1)`, which only
matches whole lexemes/words. Searching for `e` therefore never matched a post
containing the word `hey`. It also wouldn't match partial words like `every` →
`everyo`.

Now it matches the existing user-search approach in `user_store.go`:
`p.content ILIKE '%' || $1 || '%' ESCAPE '\'` with the same
`strings.NewReplacer(\→\\, %→\%, _→\_)` escaping so user-supplied wildcards are
treated literally.

## Why

The task: "search results should be generic not strict". A single letter (or
partial word) should surface any post containing it. Full-text `@@` matching
is word-boundary based and can never do substring matching, so it was replaced
with `ILIKE`.

## Files touched

<<<<<<< HEAD
- `web/src/layout/SocialMediaLayout.tsx` (sidebar Appearance box + import)
- `web/src/contexts/ThemeContext.tsx` (catalog, group union, default id)
- `web/src/components/ThemeCustomizer.tsx` (empty group removed)
- `web/src/theme-themes.css` (kept themes only + kanagawa override)
- `SUMMARY.md` (this section)

## Things a reviewer should double-check

- **Stored theme ids**: an existing user whose `localStorage["vite-ui-theme-id"]`
  holds a removed id (e.g. `zinc`) will be silently mapped to Claude on next
  load — intended, but confirm that's acceptable UX.
- **Kanagawa light mode**: the override targets the sidebar's nav text only.
  Other hardcoded colors (e.g. the search input `text-primary`) still derive
  from theme vars and read fine against the dark bg; verify in the browser.
- **Catppuccin latte light blocks** were preserved byte-for-byte; no palette
  values were "fixed" as part of this trim.
- The Settings appearance card still also has the server-persisted
  "Theme" (light/dark/system) and "Font Size" selects; those are separate
  from the `ThemeContext` customizer and were left untouched.
- Lint + `tsc -b && vite build` pass inside the worktree.

---

---

# SUMMARY — refresh-token-rotation

---
# SUMMARY — message-gradient

DM conversation bubbles now look like the iMessage/Instagram "global gradient"
chat: **every message box paints the SAME gradient, anchored to the viewport
(`background-attachment: fixed`), so the whole thread reads as one continuous
gradient and each bubble acts as a mask/window into it.** The colors are driven
by each theme's chart palette, so the effect re-themes automatically.

## What was changed and why

The conversation view (`ConversationPage.tsx`) styled outgoing bubbles with a
flat `bg-primary` and incoming with `bg-muted`. The request: give the message
boxes the iMessage/Instagram treatment where the gradient is *global* — the
bubbles are windows into a single screen-wide gradient, not individually
colored.

- **One shared gradient, painted globally.** `index.css` defines a single
  `--chat-gradient` (a 135° 4-stop gradient composed from the per-theme
  `--chart-2/1/4/5` tokens). Because it is a `var(-)`-on-`var(-)` chain defined
  once at `:root`, it resolves to each theme's own chart palette at use time
  (light and dark), so there is literally one global gradient definition whose
  colors swap with the active theme — no per-theme edits needed.
- **Bubbles = masks into that gradient.** `.chat-bubble-mine` sets
  `background-image: var(--chat-gradient); background-attachment: fixed`. With
  fixed attachment each bubble paints the gradient relative to the *viewport*,
  so as you scroll, bubbles slide through a stationary gradient and adjacent
  bubbles show contiguous slices — the "mask into a global gradient" effect.
  Border-radius clips the background to the bubble shape.
- **Two-tone, both sides on the same gradient.** Outgoing bubbles show the vivid
  gradient with white text + a soft drop shadow for contrast. Incoming bubbles
  (.chat-bubble-theirs) use the *identical* gradient washed through a 72% white
  overlay (62% black overlay under `.dark`) so the two sides share the same
  continuous gradient while staying distinguishable, and their body text keeps
  the theme's `text-primary`.
- **Graceful degradation**: iOS Safari doesn't support `background-attachment:
  fixed`, so there each bubble falls back to showing the full gradient from its
  own top-left — still gradient bubbles, just not viewport-continuous.

## Files touched

- `web/src/index.css` — added `--chat-gradient` var to the base `:root` tokens;
  added `.chat-bubble-mine`, `.chat-bubble-theirs`, and `.dark
  .chat-bubble-theirs` classes.
- `web/src/pages/ConversationPage.tsx:108-114` — bubbles now use
  `chat-bubble-mine` / `chat-bubble-theirs text-primary`; outgoing timestamp is
  `text-white/70` (was `text-primary-foreground/70`, which no longer derives
  from the flat primary color).
=======
- `server/internal/store/post_store.go` — `Search()`: swapped the full-text
  clause for the escaped `ILIKE` substring clause.
- `server/internal/handlers/integration_test.go` — added
  `TestSearchSubstringMatch` (single-letter `e` → `hey`, partial word
  `hey every` → `hey everyone`).
- `.opencode/project-notes.md` — recorded the behavior change.
>>>>>>> agent/fuzzy-search-results

## Verification

- `go test ./internal/handlers/ -run TestSearchSubstringMatch` fails on the
  old clause (0 results), passes after the fix.
- `go test ./...` all pass (including `TestSearchFilters`, `TestSearchHashtagsAndTrends`).
- `gofmt` and `go vet` clean.

## Reviewer double-checks

- The GIN index `posts_content_search_idx` (migration `000011_create-search`,
  `to_tsvector('simple', content)`) is now **unused** by the search path. It was
  left in place deliberately — it only costs write overhead, is harmless to
  reads, and removing it would require a new (parallel-branch collision-prone)
  migration. Worth deciding separately if the index should be dropped.
- Behavior is intentionally looser than before: `q=kayak` still matches all the
  same test posts (substring ⊇ full word), but now also matches partial/single
  chars and is case-insensitive (`ILIKE`). This is the requested "generic"
  behavior.