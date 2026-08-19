# SUMMARY — gaggle-goose-branding

Replaces the placeholder Vite favicon and the sidebar "G" text logo with the
goose-themed Gaggle logo, generated previously from `image.jpg`.

## What was changed and why

- The app had **no real favicon**: `web/index.html` referenced `/vite.svg`,
  but there is no `web/public/` directory, so the browser 404'd. Added a
  `web/public/` dir with two raster-traced SVG logos copied from
  `/home/bau/Programming/svg-img`:
  - `web/public/favicon.svg` ← `image_max.svg` (goose + orange `#F86600`
    background tile) — self-contained, visible in the browser tab on both
    light and dark themes.
  - `web/public/gaggle-goose.svg` ← `goose_max.svg` (transparent-background
    goose) — for the in-app sidebar mark.
- `web/index.html`: favicon `<link>` now points at `/favicon.svg`.
- `web/src/layout/SocialMediaLayout.tsx` (App Logo block): dropped the
  primary-colored circle + "G" letter glyph and replaced it with an `<img>`
  of the transparent goose (`w-10 h-10 rounded-full`); the "Gaggle"
  wordmark text is unchanged.

## Files touched

- `web/public/favicon.svg` (new)
- `web/public/gaggle-goose.svg` (new)
- `web/index.html`
- `web/src/layout/SocialMediaLayout.tsx`

## Verification

- `npm run build` (tsc + vite): passes; both SVGs emitted into `dist/`.
- `npm run lint`: 0 errors (14 pre-existing warnings, none in
  SocialMediaLayout.tsx).

## Reviewer double-checks

- Both SVGs are full `1024×1024` VTracer path traces, ~3.1MB
  (`gaggle-goose.svg`) and ~3.6MB (`favicon.svg`). The sidebar loads the
  goose SVG on first paint and browsers cache the favicon, but if payload
  size matters, re-trace at a lower resolution or ship a small PNG favicon.
- Visual QA: the goose inside `goose_max.svg` is centered with margin, so at
  40px it renders as a padded icon — confirm the crop/rounding looks right
  (I could not view rendered images from this session).
- Confirm the favicon tile looks OK against both light and dark browser
  tabs.

---

# SUMMARY — chat-ui-fixes

Three small UI fixes: long DM messages now wrap instead of overflowing into a
horizontal scrollbar, the poll vote count moved from the top to the bottom of
the poll card, and the profile action buttons reordered to Message, Follow,
three-dots.

## What was changed and why

- **Long messages wrap** (`web/src/pages/ConversationPage.tsx`): the message
  bubble is a flex child capped at `max-w-[75%]`, but an unbroken string (e.g.
  a long URL) forced it past that cap because flex items default to
  `min-width: auto` and `overflow-wrap` was never set. Added `min-w-0` to the
  bubble and `break-words` to the message body so it wraps within the 75% cap;
  newlines are still preserved (`whitespace-pre-wrap` kept).
- **Poll vote count moved to bottom** (`web/src/components/PollCard.tsx`): the
  "N votes" label was the first child of the poll card (above the options);
  it now renders as the last element, below the options and the "Poll closed"
  note.
- **Profile button order** (`web/src/pages/ProfilePage.tsx`): the Follow and
  Message buttons were swapped so the row reads Message, Follow, three-dots
  menu.

## Files touched

- `web/src/pages/ConversationPage.tsx`
- `web/src/components/PollCard.tsx`
- `web/src/pages/ProfilePage.tsx`

## Reviewer double-checks

- `min-w-0 break-words` on the DM bubble: confirm a long unbroken URL/word
  wraps on both mine/theirs bubbles, and that very short messages still render
  as compact bubbles.
- Poll vote count: confirm it sits below the options (and below "Poll closed"
  when the poll is closed).
- Profile buttons: confirm order is Message, Follow, three-dots and Follow's
  outline/default styling logic is intact after the reorder.

---

# SUMMARY — sidebar-mobile-nav

Responsive navigation cleanup. At narrow widths the left sidebar used to show a
ran of icon-only nav items whose hover pill was full-width while the icon sat
left (looked off-center), the Post button was an empty pill (label hidden but
no icon), and the icon rail rendered **at the same time** as the fixed bottom
mobile nav. Decision taken: three distinct responsive tiers instead of the
previous "both at once" behavior.

## What was changed and why

- **Three-tier nav** (`web/src/layout/SocialMediaLayout.tsx`): the left sidebar
  is now `hidden md:block` with `md:col-span-2 lg:col-span-2`, so it only
  exists from `md` (768px) up. Below `md` the app goes fully into the mobile
  design: fixed bottom nav is the ONLY navigation and the main column is full
  width. At `md`–`lg` the sidebar is an icon-only rail; at `lg+` it shows the
  full labels with the right rail, unchanged.
- **Grid math preserved**: base = main `col-span-12`; `md` = 2+10; `lg` =
  2+7+3 (sidebar + main + right rail). No overlapping/double nav at any width.
- **Icon-only hover centering**: `NavItem` switched from
  `justify-start items-center space-x-4` to
  `justify-center lg:justify-start gap-x-4`, so when the label is hidden
  (below `lg`) the icon is horizontally centered inside the full-width hover
  background; when labels show it stays left-aligned. The Post button, logo
  wordmark, and user dropdown similarly use `justify-center lg:justify-start/block`
  for the icon-only state.
- **Post button write icon**: instead of an empty pill (below `md` the sidebar
  doesn't render; at `md`–`lg` the label was hidden with no icon), the button is
  now `md:w-14 md:h-14 md:px-0` circle containing a `PenSquare` icon and returns
  to the full-width pill + "Post" label at `lg+`.
- **Mobile bottom nav gained destinations** (`web/src/components/MobileNavigation.tsx`):
  since below `md` the sidebar is gone, Explore and DMs (Messages) were added to
  the sidebar-less bottom nav (previously missing), and the DM unread badge now
  shows. Kept: Home, Alerts, compose FAB, Saved, Profile, Settings.
- **Mobile bottom padding**: main column gets `pb-16 md:pb-0` so the fixed
  bottom nav doesn't cover the last content.

## Files touched

- `web/src/layout/SocialMediaLayout.tsx`
- `web/src/components/MobileNavigation.tsx`

## Review notes

- Below `md` the sidebar (and therefore the account dropdown with Log out,
  and Admin/Lists/Mentions links) is gone. Logout is only reachable via that
  dropdown. If losing Log out on phones is unacceptable, it should be added to
  the Settings page or the mobile nav — flagging for review.
- The `md`–`lg` icon rail still shows badge dots on Messages/Notifications;
  verify badge position against the icon-only layout.
- No server/DB changes; frontend-only.

---

Follow-up to the catalog trim: cut the Catppuccin catalog down to one flavor
(mocha), gave Kanagawa a real light mode, removed the manual "Rounded corners"
slider (radius now always comes from the theme), and made the selected state
in the theme/font pickers visually distinct from hover so selection reads
clearly.

## What was changed and why

- **Catppuccin flavor collapse** (`web/src/contexts/ThemeContext.tsx`,
  `web/src/theme-themes.css`): Macchiato and Frappé are near-indistinguishable
  from Mocha, so the catalog went from 9 → 3 Catppuccin themes: `Mocha`,
  `Mocha Blue`, `Mocha Peach`. Kept the `catppuccin-mocha-*` ids so stored
  theme ids keep working; dropped `catppuccin-macchiato-*` / `catppuccin-frappe-*`
  CSS blocks are gone (verified absent from the production CSS bundle). Any
  stored macchiato/frappe id now falls back to Claude via `findTheme`.
- **Kanagawa light/dark now actually differ** (`web/src/theme-themes.css`):
  previously `:root` (light) and `.dark` were byte-for-byte the same dark
  palette, so toggling light/dark did nothing. `:root[data-theme="icon-kanagawa"]`
  is now a real light "washi" palette (paper/ink/wisteria) and the dark block is
  the wave palette. The old `.text-gray-800` nav-label override was deleted —
  with a light sidebar it's no longer needed.
- **Rounded corners slider removed** (`web/src/components/ThemeCustomizer.tsx`,
  `web/src/contexts/ThemeContext.tsx`): the slider ("Rounded corners") was
  removed and the whole radius override mechanism (state, `setRadius`,
  `vite-ui-radius` localStorage, the `--radius` effect) was deleted. Radius is
  now set once from `findTheme(themeId).defaultRadius` when the theme changes —
  the theme is the single source of truth. Per-theme `--radius` values in
  `theme-themes.css` are now redundant but harmless.
- **Selected state ≠ hover** (`web/src/components/ThemeCustomizer.tsx`): theme
  and font buttons previously highlighted selection with the same ring used on
  hover, so it was easy to mistake hover for "editor has kanagawa / font has
  inter". Selected buttons now get `bg-primary/10` + `font-medium` so selection
  is unmistakable while hover stays just a border tint.

## Files touched

- `web/src/contexts/ThemeContext.tsx` (catalog + radius removal)
- `web/src/components/ThemeCustomizer.tsx` (radius slider removed, selected styling)
- `web/src/theme-themes.css` (catppuccin trim, kanagawa light/dark palettes)
- `SUMMARY.md` (this section)

## Things a reviewer should double-check

- **Kanagawa light palette** is hand-derived from the wave/"washi" reference
  palette — verify contrast on the sidebar nav, feed text, and the DM bubble
  gradient (which derives from `--chart-*`) in light mode in a browser.
- **Stored ids**: a user with `vite-ui-theme-id` set to a removed macchiato/
  frappe id silently falls back to Claude. Radius localStorage keys
  (`vite-ui-radius`) are simply ignored now.
- Lint + `tsc -b && vite build` pass inside the worktree; no server or
  migration files were touched.

---


---

# SUMMARY — profile-action-buttons-align

Fixes the profile action buttons on someone else's profile (Follow/Unfollow,
Message, three-dots menu) appearing centered instead of flush right.

## Root cause

`ProfilePage.tsx` always rendered an "Edit profile" button at the end of the
action-button row and just hid it for other viewers via
`${isCurrentUser ? "visible" : "invisible"}`. `visibility: hidden` keeps the
element in the layout, so on any other user's profile the invisible
~106px-wide "Edit profile" button still occupied the rightmost slot of the
`flex justify-end` row. The three visible buttons were therefore pushed left
by that reserved space and stopped ~114px short of the right edge — reading as
"centered". On narrow screens it also overflowed the avatar/column to the
left (probe at 375px showed the Unfollow button starting at x=-84 before the
fix). The invisible button had existed since the original frontend commit
(git blame `^325aae4`), so the bug was unrelated to the profile-tabs merge.

## Change

`web/src/pages/ProfilePage.tsx`: render the "Edit profile" button only when
`isCurrentUser` is true (`{isCurrentUser && <Button …>Edit profile</Button>}`)
instead of always rendering it invisibly. For other users the row now holds
exactly the visible buttons and `justify-end` lands them flush against the
container's right edge. Current-user layout is unchanged (still just "Edit
profile", right-aligned).

## Verification

- Playwright probes (`/profile/bob`, logged in as alice): before the fix the
  visible buttons ended ~114px short of the row's right edge at every tested
  viewport; after, the three-dots button ends exactly at the container's
  right edge at 375 / 640 / 768 / 1024 / 1280 / 1920 px, and the 375px
  overflow-to-the-left is gone.
- Self-profile still shows a single right-aligned "Edit profile" button;
  other-profile has zero "Edit profile" buttons rendered.
- `npm run lint`: 0 errors (14 pre-existing warnings, none in ProfilePage).
- `npm run build` (tsc -b + vite): passes.

## Files touched

- `web/src/pages/ProfilePage.tsx`

## Review notes

- No backend, tests, or migrations affected. The `invisible`→conditional
  swap is the only behavioral change; row height is unchanged (the remaining
  "Edit profile" / action buttons are the same default height).

---

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


---

# fuzzy-search-results

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

- `server/internal/store/post_store.go` — `Search()`: swapped the full-text
  clause for the escaped `ILIKE` substring clause.
- `server/internal/handlers/integration_test.go` — added
  `TestSearchSubstringMatch` (single-letter `e` → `hey`, partial word
  `hey every` → `hey everyone`).
- `.opencode/project-notes.md` — recorded the behavior change.

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

Moved every appearance/theme control out of the right-rail "Appearance" box
into the existing **Settings → Appearance** card (which already embedded the
`ThemeCustomizer`), and slimmed the theme catalog down to the curated set
(Claude, Caffeine, Perplexity, all 9 Catppuccin flavors, Kanagawa, Comic,
Neo-brutalism) plus the category/editor themes already dropped (Classic,
other studio-* brands, other editor themes, Sketch, Arcade, Retro Terminal).

## What was changed and why

- **Appearance box removed from the right sidebar**
  (`web/src/layout/SocialMediaLayout.tsx`): the `<div class="bg-muted …">`
  "Appearance" card with `<ThemeCustomizer />` was deleted (and its import).
  All of its controls already live in `SettingsPage`'s Appearance card via
  the same `<ThemeCustomizer />`, so nothing was lost — the right rail now
  only has Search / Trending / Who to follow.
- **Catalog trimmed to the kept list** (`web/src/contexts/ThemeContext.tsx`):
  `THEME_CATALOG` went from 38 → 15 entries. Kept groups: `Brands`
  (studio-claude / studio-caffeine / studio-perplexity), `Catppuccin`
  (all mocha/macchiato/frappe × mauve/blue/peach), `Editor` (icon-kanagawa),
  `Fun` (fun-neobrutalism / fun-comic). The `"Classic"` group and its 12
  shadcn schemes are gone, along with the `ThemeDefinition.group` union
  member.
- **Empty catalog group removed** (`web/src/components/ThemeCustomizer.tsx`):
  `groups` no longer renders the empty "Classic" heading.
- **Default theme repointed** (`ThemeContext.tsx`): `DEFAULT_THEME_ID` was
  `"slate"` (removed); it is now `"studio-claude"`. `findTheme` falls back to
  it, so users with a removed theme id in `vite-ui-theme-id` localStorage now
  resolve to Claude instead of crashing on the non-null assertion.
- **Dead CSS removed** (`web/src/theme-themes.css`): rewrote the file to
  retain only the 15 kept themes (light+dark blocks and their scoped
  overrides), going from 3406 → ~1158 lines. Verified the production build
  emits no selectors/classes for the dropped themes.
- **Kanagawa light-mode fix**: Kanagawa only ships its dark wave palette, so
  in light mode the (dark) sidebar background made the hardcoded
  `text-gray-800` nav labels ("Home / Explore / …") almost unreadable. Added
  a light-mode-only override scoped to kanagawa:
  `:root[data-theme="icon-kanagawa"]:not(.dark) .text-gray-800 { color:
  var(--sidebar-foreground) }`. It targets only the non-dark state so
  `dark:text-gray-100` still wins in dark mode.

## Files touched

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

## Verification

- `npm run build` (web-tools container) — tsc + vite pass.
- `npm run lint` — 0 errors, only the same 14 pre-existing react-refresh
  warnings as the base branch.
- Inspected the compiled `dist` CSS: `--chat-gradient`, `.chat-bubble-mine`
  (with `background-attachment:fixed`), `.chat-bubble-theirs`, and `.dark
  .chat-bubble-theirs` all present with the expected values.
- Not browser-verified: other agent worktrees share the single Docker compose
  stack, and rebuilding `web` would clobber another session's running build, so
  the shared containers were left untouched.

## Things a reviewer should double-check

- **Eyeball the effect** in a browser (rebuild `web` from this branch when the
  stack is free): check a thread in light + dark mode and in a couple of themes
  (e.g. zinc and fun-comic) — chart-palette colors vary a lot per theme, so the
  gradient's vibe changes per theme by design. Confirm outgoing text stays
  legible over the lightest chart colors (the `0 1px 2px rgb(0 0 0 / .35)`
  shadow helps).
- **`background-attachment: fixed` inside a scroll container**: the message
  list is an `overflow-y-auto` div with no transform/opacity/filter ancestors,
  so bubbles keep true viewport-fixed painting on desktop browsers. If a future
  layout change adds a `transform`/`backdrop-filter` ancestor, the "global"
  look silently degrades to per-bubble gradients — keep that in mind.
- **Incoming-bubble wash values** (72% white light / 62% black dark) are
  hand-tuned for readability; tweak if a theme's chart palette is unusually
  light/dark.
- The gradient intentionally applies only to DM conversation bubbles; the
  Messages inbox list and other `bg-primary` surfaces (buttons, badges) are
  untouched.

---

---
# SUMMARY — detailed-search-filters

Post search (`GET /search?type=posts`) now supports fine-grained filters, and
the Search page exposes them as a collapsible, URL-driven filter panel. The
hashtag page (`/hashtags/{tag}/posts`), trends, and user search are unchanged.

## What was changed and why

Search previously accepted only a free-text `q`. This adds six optional query
params, each an additive SQL clause on the existing keyset-paginated
`postStore.Search`:

- `from=<username>` — posts authored by that user (exact username match)
- `hashtag=<name>` — posts that also contain that hashtag (normalized: case
  folded, leading `#` stripped — the same normalization hashtag writes use)
- `has_media=true` — only posts with at least one attached media row
- `min_likes=<n>` — posts with `likes_count >= n` (denormalized count, no join)
- `include_replies=true` — include replies; default remains top-level only
- `since` / `until` — `created_at` range (inclusive start, exclusive end);
  accepts RFC3339 or `YYYY-MM-DD` (a date-only `until` covers the whole day)

Invalid `since`/`until` values and `until <= since` return 400.

Frontend: `SearchPage` gained a "Filters" toggle that opens a panel (from user,
hashtag, min likes, date from/to, media-only and include-replies switches) with
Apply/Clear. Filters are committed to the URL params (`from`, `hashtag`,
`media`, `min_likes`, `replies`, `since`, `until`) so they are shareable and
bookmarkable, and are part of the react-query key (`['search-posts', q, json]`)
so applying filters refetches. The `useSearchPosts(query, filters)` default
keeps `ExplorePage` working unchanged.

## Files touched

- `server/internal/models/search.go` — new `PostSearchFilters` struct
- `server/internal/store/post_store.go` — `Search` builds WHERE clauses
  dynamically from the filters (still top-level-only by default)
- `server/internal/store/store.go` + `server/internal/service/service.go` —
  `Search` interface signatures updated to take the filters
- `server/internal/service/search_service.go` — validates `until > since`
- `server/internal/handlers/search_handler.go` — parses the new query params;
  `parseSearchFilters` / `parseSearchTime` helpers
- `server/internal/handlers/integration_test.go` — `TestSearchFilters` covers
  every filter, combinations, the date range, and the 400 on bad dates
- `web/src/api/search.ts` — `PostSearchFilters` type + `searchPosts` params
- `web/src/hooks/useSearch.ts` — filters folded into the query key
- `web/src/pages/SearchPage.tsx` — collapsible filter panel, URL-driven

## What a reviewer should double-check

- **SQL correctness**: `from`/`hashtag` use `EXISTS` subqueries against
  `$n` placeholders; cursor clauses appended by `listDiscoverablePosts` use
  `len(args)+1`. A filter clause after hashtag/from would shift `$n` — verify
  the arg-index bookkeeping in `postStore.Search` holds for all six filters.
- **Search term interpretation**: `hashtag` is AND-ed with the full-text `q`
  (a hashtag-only search needs the term in `q` too). If empty-content hashtag
  searches should work, that's a follow-up.
- **`until` date-only semantics** (end-of-day inclusive) vs RFC3339 exclusive
  bound — picked deliberately; confirm it matches expectations.
- **Toggle params parity**: `SearchPage` writes `media`/`replies`; the API
  reads `has_media`/`include_replies`. The mismatch is bridged in
  `web/src/api/search.ts` — a reviewer may prefer one naming for both.
- **Badge hydration** for user search is untouched; post hydration reuses the
  shared `hydrateFeed`.

---
# SUMMARY — pin-unpin-timeline-bug

Pin/unpin from the timeline left the "Pin to profile / Unpin from profile" menu
stuck on the old value (unpin never flipped the button, pinning a different
post didn't flip the new one), while the same action from one's own profile
worked. Root cause was the timeline's **Redis-cached home feed** plus a client
that had **no optimistic pin update**.

## What was changed and why

**1. Root cause of the timeline/profile difference (server, already in code but
never actually serving).**
`GET /posts/feed` is served from a 60s Redis cache; `is_pinned` lives inside
each cached home-feed payload. The `pinned-post-menu-fixes` merge added
`invalidateFeedForUserAndFollowers` to `PinPost`/`UnpinPost`/`UpdatePost`/
`DeletePostByID`, but the live stack never ran that build: HEAD's migrations
contained **two files numbered 000016** (`000016_add-mute-relationship` and
`000016_add-refresh-token-session`), so golang-migrate refused to open the
migration source and the `api` container crash-looped on startup — anything the
user tested necessarily ran an older api image without the feed invalidation.
With no invalidation, the timeline kept serving the previous `is_pinned` flags
for the cache TTL; and because the client feed query is `staleTime: Infinity`
with `refetchOnWindowFocus: false`, the stale copy latched indefinitely — "the
button never updates". The profile was unaffected because `/users/{u}/pinned`
and `/users/{u}/posts` hit the DB directly.

- **Renumbered the migration** `000016_add-refresh-token-session.{up,down}.sql`
  → `000017_add-refresh-token-session.{up,down}.sql` so HEAD boots (DB state was
  only ever at migration 15, so nothing renumbered needed backfilling).
- **Rebuilt + verified the server against the live stack**: after `DELETE
  /posts/{id}/pin` or `POST /posts/{id}/pin`, the very next `/posts/feed`
  response carries the correct `is_pinned` flags (Redis invalidation effective).

**2. Client hardening (the actual code fix).** The menu label is driven by
`post.is_pinned`, which only changed after a successful feed refetch. Pin/unpin
had **no optimistic update** (unlike like/repost/bookmark) and no error
rollback, so a slow refetch — or a briefly stale server cache — made the button
appear stuck. In `web/src/hooks/usePost.ts`:

- `usePinPost.onMutate` now optimistically flips `is_pinned` on **every cached
  copy** of the post (`updatePostInAllQueries`), cancelling in-flight single-post
  fetches first — the button flips instantly, before the request or refetch
  resolves.
- `usePinPost.onError` flips it back (rollback).
- Extended `POST_QUERY_KEYS` with `'search-posts'`, `'hashtag-posts'`,
  `'list-feed'` so posts render in those surfaces too and the optimistic
  engagement/pin/author updates stay consistent everywhere the post card lives.

## Files touched

- `web/src/hooks/usePost.ts` — optimistic pin flip + rollback; `POST_QUERY_KEYS`
  extended with search/hashtag/list feeds.
- `server/cmd/migrate/migrations/000016_add-refresh-token-session.{up,down}.sql`
  → renamed to `000017_...` (duplicate migration version that blocked `api`
  startup).

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` all pass (handler
  integration suite covers pin/unpin flows; the harness runs without Redis).
- `npm run build` passes; `npm run lint` reports the same 14 pre-existing
  warnings as base, zero new.
- Live stack (rebuilt from this branch): curl repro shows `/posts/feed` reflects
  pin state immediately after pin/unpin (cache invalidated).
- Playwright (host chrome) on the timeline:
  - full cycle flips labels correctly — unpin #61 → "Pin to profile"; pin #40 →
    #40 "Unpin from profile" AND #61 "Pin to profile"; restore works;
  - optimistic test (requests artificially delayed) — label flips *before* the
    response refetch completes;
  - rollback test (unpin fails with 500) — label flips optimistically then
    reverts to "Unpin from profile" on error.

## Things a reviewer should double-check

- **Migration renumber**: the new 000017 files were never applied anywhere
  (migrate before refused to open the source; DB was last at version 15). On
  real deploys, confirm the refresh-token-session migration runs cleanly.
- **`POST_QUERY_KEYS` extension** changes behavior of ALL optimistic engagement
  updaters (like/repost/bookmark/author updates) — they now also update
  search/hashtag/list feed caches. Shapes are identical (`Envelope<PostFeed>`);
  validates fine via `setQueriesData` prefix matching (`['search-posts', q]`,
  `['hashtag-posts', tag]`, `['list-feed', id]`).
- Redis staleness is not exercised by `go test` (test harness passes `nil` rdb,
  no seam for a fake). The live-stack repro + browser probes are the regression
  check; see `.opencode/project-notes.md` "home feed Redis cache".
- The running Docker stack now serves this branch's build (shared single
  compose stack); other parallel agent sessions will see their branch's build
  replaced.

---


---

# SUMMARY — profile-relationships-view

Fix so that clicking the "Following"/"Followers" counts on a user's profile lets
you view that user's actual follow relationships.

## Root cause

The follow-list feature already existed in source (routes
`/profile/:username/followers|following`, `FollowListPage`, backend
`GET /users/{username}/followers|following` returning the flat
`items: UserProfileResponse[]` array). Two things stopped it working in a real
deployment:

1. **Stale running stack.** The `api` container predated the merge of
   `fix-profile-tabs-and-user-relations`, which changed these two endpoints from
   nested `followers:`/`following:` objects to the app-wide flat `items:` array.
   The web bundle expected `items`, so `res.data.items` was `undefined` and
   `FollowListPage` crashed into a blank screen — "clicking following/followers
   doesn't show relationships".
2. **Duplicate migration version.** Two merged branches both created a migration
   numbered `000016` (`add-mute-relationship` from fix-profile-tabs…, and
   `add-refresh-token-session` from refresh-token-rotation). golang-migrate aborts
   with "duplicate migration file", so a fresh deploy of current main could not
   boot the api container at all.

## What was changed and why

- Renumbered `000016_add-refresh-token-session.{up,down}.sql` →
  `000017_add-refresh-token-session.{up,down}.sql`. The two 000016 files were
  independent (mute relationship type vs refresh-token session columns); keeping
  mute at `000016` and the refresh-token migration at `000017` matches the order
  the branches were merged. The dev DB was at version 15, so 16 and 17 apply
  cleanly.
- Rebuilt the `api` and `web` containers from current source (the deployment
  fix for the stale-api symptom). No `FollowListPage`/handler code changed — it
  was already correct for the flat `items` contract.

## Verification

- `docker compose up --build -d api web` boots healthy; `schema_migrations`
  shows versions through 17.
- `GET /api/v1/users/alice/following` and `/followers` return flat `items`
  with viewer-relative `is_following/is_blocked/is_muted` and badges.
- Playwright (host chrome, "Test sign in"): `/profile/alice` shows
  "2 Following / 3 Followers"; clicking Following lists Charlie Brown and
  Bob Smith (each with a Follow/Following button); clicking Followers lists
  Grace, Charlie, Bob. No console errors.
- `go test ./...` passes (incl. relationship suites in handlers).
- Frontend `npm run build` (tsc + vite) passes; `npm run lint` reports 0 errors.

## Files touched

- `server/cmd/migrate/migrations/000016_add-refresh-token-session.up.sql` → renamed to `000017_add-refresh-token-session.up.sql`
- `server/cmd/migrate/migrations/000016_add-refresh-token-session.down.sql` → renamed to `000017_add-refresh-token-session.down.sql`

## Things a reviewer should double-check

- Migration content is unchanged — only the file names / version number.
- Any environment that already applied the refresh-token-session migration as
  `000016` would diverge from the new numbering. The dev DB (this session) was
  at version 15 and had not applied either 000016, so 16/17 apply cleanly here;
  confirm no CI/prod DB applied it under the old number before this change.
- The web/`FollowListPage` code was intentionally untouched (already correct);
  the visible bug was a stale deployment, so a rebuild is the real fix.

---

# SUMMARY — tag-users-in-posts

Users can now be tagged in a post by writing `@username`, mirroring the
hashtag feature end-to-end: tags are parsed and stored at write time, `@user`
renders as a link to their profile everywhere post content is shown (feed,
post page, and the compose live-highlight), and there is a `/mentions` page
listing the posts that tagged **you**.

## What was changed and why

Mention *notifications* already existed (create + quote, wired in the post
handlers), but nothing else did: `@username` was plain text, mentions weren't
stored or queryable, and there was no feed. This change adds the missing
hashtag-parallel pieces.

**Server**
- Migration `000017_add-post-mentions`: `post_mentions (post_id, user_id)`
  composite PK, both FKs `ON DELETE CASCADE`, index on `(user_id, post_id)`.
  No catalog table needed (unlike hashtags) because users are first-class —
  mentions reference `users` directly.
- `postutil.ExtractMentions`: unicode-aware `@username` extraction
  (`(?:^|[^\pL\pN_])@([\pL\pN_]{1,16})`), lowercased + deduped, unknown users
  dropped at resolution time.
- `mentionStore.SyncPost` runs inside the post transaction at create, update,
  and quote (`post_service.go`), right beside `Hashtags.SyncPost`. Resolution
  is case-insensitive (`LOWER(username) = LOWER($1)`) and excludes soft-deleted
  users.
- `GET /mentions` (authenticated): keyset-paginated feed of posts mentioning
  the viewer, hydrated via the shared `search_service.hydrateFeed` (engagement,
  polls, media, parents). Reuses `postStore.listDiscoverablePosts` like
  `ListByHashtag`; top-level posts only (parity with hashtag feeds).
- Swagger annotation added for the new endpoint (`make swag` regenerated —
  only `/mentions` was added to `server/docs`).

**Frontend**
- `HashtagText.tsx` replaced by `ContentLinks.tsx`, which renders both `#tag`
  (→ `/hashtags/tag`) and `@user` (→ `/profile/user`) as accent-colored links.
  Used by `FeedPost` (covers the post page too) and `ComposeContent`'s
  live-highlight mirror. Composer CSS class renamed `.hashtag-composer` →
  `.composer-highlight`.
- New `MentionsPage` at `/mentions` (mirrors `HashtagPage`), nav entry in the
  sidebar, `getMentionsFeed` API + `useMentionsFeed` infinite-query hook.

## Files touched
- Server: `server/cmd/migrate/migrations/000017_add-post-mentions.{up,down}.sql`
  (new), `server/internal/postutil/mentions.go` (new),
  `server/internal/store/mention_store.go` (new),
  `server/internal/store/{store.go,post_store.go}`,
  `server/internal/service/{post_service.go,search_service.go,service.go}`,
  `server/internal/handlers/{search_handler.go,integration_test.go}`,
  `server/internal/api/router.go`, `server/docs/*` (regenerated).
- Frontend: `web/src/components/ContentLinks.tsx` (new),
  `web/src/components/HashtagText.tsx` (deleted),
  `web/src/pages/MentionsPage.tsx` (new), `web/src/components/{FeedPost,ComposeContent}.tsx`,
  `web/src/index.css`, `web/src/api/search.ts`, `web/src/hooks/useSearch.ts`,
  `web/src/App.tsx`, `web/src/layout/SocialMediaLayout.tsx`.

## Verification
- `go build ./...`, `go vet ./...`, full `go test ./...` green; new
  `TestMentionsFeed` covers case-insensitive tagging, bogus-username dropping,
  a non-mentioned user's empty feed, reply-mentions exclusion, and
  mention removal on edit.
- Frontend `tsc -b && vite build` and `eslint .` pass (0 errors; only
  pre-existing warnings outside changed files).

## Reviewer double-checks
1. **Notification/storage regex divergence**: mention *notifications*
   (`notification_service.go:20`) use ASCII `@([A-Za-z0-9_]{3,16})\b`, while
   storage uses unicode + case-insensitive resolution. A unicode username (or
   `@Name` casing) is stored and rendered as a link but may not generate a
   notification. Left as-is to avoid touching the existing notification
   behavior — consider aligning them in a follow-up.
2. **Top-level-only mentions feed** (parity with hashtag feeds): a reply that
   tags you appears in your mention *notifications* but not in `/mentions`.
   Confirm that's the desired semantic.
3. **`GetByUsername` is case-sensitive**; mention resolution
   intentionally bypasses it with a `LOWER()` query so `@Name` resolves
   regardless of stored casing. `GetUserProfileByUsername` is case-insensitive,
   so the rendered `/profile/...` link always resolves.
4. **Feed caching**: mentions are stored per-post and don't affect the home-feed
   Redis cache; no invalidation changes were needed.
5. **Mobile nav** was intentionally left unchanged (bottom bar is full) —
   `/mentions` is reachable via the desktop sidebar. Add a mobile entry if
   desired.
6. **Swag**: `search_handler.go` now imports `models` with a `var _ models.PostFeed`
   dummy (the settings_handler trick) — swag can't otherwise resolve
   `models.Envelope` in annotations.

---# SUMMARY — refresh-token-rotation
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

**8. "Gaggle" brand text overflowed its sidebar column (~768–1220px)**
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
