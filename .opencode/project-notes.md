## Theme catalog trim + kanagawa light mode (agent/move-themes-to-settings)
- `THEME_CATALOG` (web/src/contexts/ThemeContext.tsx) is now 15 themes: claude /
  caffeine / perplexity / 9× catppuccin / kanagawa / neobrutalism / comic. The
  `"Classic"` group is gone; `DEFAULT_THEME_ID` is now `"studio-claude"`
  (was `"slate"`, removed). A stored removed id in `localStorage` maps to Claude
  via `findTheme` fallback.
- Kanagawa only ships its dark "wave" palette — `:root` (light) == `.dark`. Nav
  labels are hardcoded `text-gray-800 dark:text-gray-100`, so kanagawa light mode
  needed an override: `:root[data-theme="icon-kanagawa"]:not(.dark) .text-gray-800
  { color: var(--sidebar-foreground) }`. Scoped to `:not(.dark)` so it can never
  shadow `dark:text-gray-100`.
- The right-rail "Appearance" card was removed; appearance controls live only in
  Settings → Appearance. Theme CSS went from 3406 → ~1158 lines (dead themes
  deleted). Build output confirmed no dropped-theme selectors remain.

## DM bubbles: global gradient via background-attachment: fixed
- Every outgoing/incoming conversation bubble (`ConversationPage.tsx`) paints
  ONE `--chat-gradient` (composed from per-theme `--chart-2/1/4/5`) with
  `background-attachment: fixed`, so jointly they read as a single
  viewport-global gradient (iMessage/Instagram "mask" trick). Bubbles must keep
  NO `transform`/`filter`/`backdrop-filter` ancestor, or browsers switch fixed
  attachment to scroll/local and the global continuity silently breaks.
- iOS Safari does NOT support `background-attachment: fixed` → falls back to
  scroll → each bubble shows the gradient from its own top-left (still gradient
  bubbles, just not viewport-continuous).
- `--chat-gradient` is defined ONCE at `:root` in `index.css` as a var chain
  over chart vars; because it's a custom-property reference it re-resolves per
  theme+mode automatically — no per-theme gradient edits needed.

## Post search substring matching (fuzzy-search-results)
- Post search no longer uses `to_tsvector @@ plainto_tsquery` (whole-lexeme
  match — searching "e" never matched "hey"). It now matches user search:
  `p.content ILIKE '%' || $1 || '%' ESCAPE '\'` with the same
  `strings.NewReplacer(\,\,%,%_,_)` escaping in `post_store.go:Search`.
  The GIN `posts_content_search_idx` (migration 000011) is now unused by the
  search path but left in place (not harmful; removing would need a migration).

## Search filters (detailed-search-filters)
- `GET /search?type=posts` filter params: `from`, `hashtag`, `has_media`,
  `min_likes`, `include_replies`, `since`, `until`. Frontend URL params are
  `media`/`replies` while the API uses `has_media`/`include_replies` — bridged
  in `web/src/api/search.ts`, don't "align" one side blindly.
- `postStore.Search` builds additive clauses with hand-rolled `$n` indexing;
  `listDiscoverablePosts` appends the cursor AFTER those args, so a new filter
  must use `len(args)+1` like the others. Watch arg order when adding filters.
- `hashtag` filter is AND-ed with the text `q`; search handlers carry no
  swagger annotations (the endpoint never appears in `server/docs`).
- Date bounds: RFC3339 or `YYYY-MM-DD`; a date-only `until` means end-of-day
  (inclusive), `since` means midnight. `TestSearchFilters` covers all filters.

## Migrations: parallel-branch version collisions
- `make new-migration` (`create -seq`) numbers from "highest in THIS branch" —
  parallel branches from the same base pick the SAME next number → collisions.
  A collision is often a **git-clean merge** (two different files sharing a
  version merge without conflict) that then crash-loops the api at boot
  ("duplicate migration file"). This is the #1 /merge-all failure.
- **Dup check** (count families per version, NOT files — up+down is normal):
  `git ls-tree -r --name-only HEAD server/cmd/migrate/migrations/ | sed -E 's|.*/||; s/\.(up|down)\.sql$//' | sort -u | awk -F_ '{c[$1]++} END {for (v in c) if (c[v]>1) print "DUP: " v}'`
- **Fix at merge**: renumber the incoming branch's file to the next free
  version (git mv + commit on the branch) BEFORE merging; verify post-merge.
  See `agent-branch-workflow` skill for the full /merge-all procedure.
- If master itself has a duplicate, fix it with ONE dedicated branch and merge
  it first; never let each branch "fix" it independently (they all pick the
  same replacement number). `/new-task` must verify master is clean first.

## Migrations & follow lists
- **Migration 000016 was duplicated on main**: `fix-profile-tabs-and-user-relations`
  merged `000016_add-mute-relationship` while `refresh-token-rotation` merged
  `000016_add-refresh-token-session` → golang-migrate dies with "duplicate
  migration file" and the api container can't boot. Resolved by renaming
  refresh-token-session to `000017`. Watch for this whenever two agent branches
  add migrations in parallel (they both start at the same `0000NN`).
- **Stale containers masquerade as bugs**: the shared compose api/web get
  replaced on rebuild from whichever worktree builds last. After merges, the
  running `api` may still serve an older response shape (e.g. nested
  `followers:`/`following:` instead of flat `items:`) while a newer `web` bundle
  expects `items` → `FollowListPage` crashes to a blank screen ("clicking
  following/followers does nothing"). Rebuild both from current source before
  debugging "broken" features.

## User @mentions / tagging (tag-users-in-posts)
- Migration `000017` adds `post_mentions (post_id, user_id)` (composite PK, both FK
  `ON DELETE CASCADE`, index on `(user_id, post_id)`). No catalog table — users are
  first-class, unlike hashtags. `mentionStore.SyncPost` runs at create/update/quote
  in `post_service.go`, right after `Hashtags.SyncPost`.
- `postutil.ExtractMentions` regex: `(?:^|[^\pL\pN_])@([\pL\pN_]{1,16})` (unicode,
  min 1 — resolution filters to real users). It is a SUPERSET of the *notification*
  regex in `notification_service.go:20` (`@([A-Za-z0-9_]{3,16})\b`, ASCII, min 3) —
  a unicode or 1-2-char username can be STORED as a mention but never produce a
  mention notification. Known divergence, kept as-is.
- **`GetByUsername` (user_store.go) is CASE-SENSITIVE** (`WHERE username = $1`) — do
  NOT use it to resolve mentions. `mentionStore.SyncPost` resolves inline with
  `LOWER(username) = LOWER($1) AND soft_deleted = FALSE` (matches the case-insensitive
  unique index). `GetUserProfileByUsername` *is* case-insensitive (line 196), so
  frontend `@Name` → `/profile/Name` links always resolve.
- `GET /mentions` = viewer-scoped mentions feed, keyset-paginated via
  `postStore.ListMentionedBy` → `listDiscoverablePosts`, **top-level only**
  (`parent_id IS NULL`), mirrors ListByHashtag. Replies linking you appear in your
  mention *notifications* but NOT the mentions feed — parity with hashtags.
- Frontend: `HashtagText.tsx` was replaced by `ContentLinks.tsx` (renders both
  `#tag` → `/hashtags/tag` and `@user` → `/profile/user`); used by FeedPost + the
  ComposeContent live-highlight mirror. Composer CSS class renamed
  `.hashtag-composer` → `.composer-highlight`.
- Swag gotcha (again): annotations using `models.X` need the file to import
  `models` — `search_handler.go` now has a `var _ models.PostFeed` dummy (mirrors
  `settings_handler.go`'s `var _ models.UserSettings`) or `models.Envelope` won't
  resolve and `make swag` exits 1.

## Server: home feed Redis cache + pin/edit/delete invalidation
- Migrations must have UNIQUE 6-digit versions — golang-migrate refuses to open
  the source on duplicates and the `api` container crash-loops at startup
  (happened with two `000016_*` files: mute-relationship + refresh-token-session;
  renumbered the latter to 000017). `docker compose build` can also serve STALE
  image layers (cache hit on reused `latest` tag) — use `--no-cache` and verify
  baked-in artifacts when a build seems to ignore edits.
- `usePinPost` flips `is_pinned` optimistically across all cached copies
  (`updatePostInAllQueries`, onMutate) + rolls back onError — the pin menu label
  is driven by `post.is_pinned` inside feed payloads, so without the optimistic
  flip a slow/stale refetch makes the button "never update".
- `GET /posts/feed` is served from a 60s Redis cache (`feed:home:{userID}:{cursor}`,
  `handlers.GetHomeFeed`). Any write that changes `is_pinned`, content, or post
  existence must invalidate via `PostHandler.invalidateFeedForUserAndFollowers`
  or the timeline serves stale JSON for up to a minute.
- PIN BUG (fixed): `PinPost`/`UnpinPost`/`UpdatePost`/`DeletePostByID` never
  invalidated the cache, so after pinning a new post the main timeline kept
  showing the OLD pinned flags — the newly pinned post's menu never gained
  "Unpin from profile". `CreatePost` and the engagement handler already
  invalidated; the four write handlers did not — now they all do.
- The non-cached paths keep the cache fresh on the profile: user feed, pinned
  endpoint (`/users/{u}/pinned`), and single-post fetches all hit the DB, which
  is why only the main timeline looked stale.
- Test harness (`testutil.NewApp`) passes `nil` rdb, so Redis staleness is NOT
  covered by integration tests — cache client is concrete (`*cache.Client`), no
  seam for a fake. Verify cache behavior against the live stack instead.



## Post thread / verification
- `postStore.GetDescendants` sorts replies `created_at DESC` (newest first)
  since the post-thread-and-bookmark-fixes change; the paged query uses
  `created_at < $cursor`. Only the single-post endpoint consumes it.
- Profile route is `/profile/:username` — NOT `/:username`. `/alice` renders
  nothing ("No routes matched").
- **`POST /auth/refresh-token` returns 500 when the cookie is missing**
  (`auth_handler.go` maps `http.ErrNoCookie` to `InternalServerError`), so every
  fresh page load with no refresh cookie logs a 500. The frontend swallows it
  during AuthContext bootstrap. Ideally return 401. (Not task-related, observed
  while testing.)
- Browser verify recipe: `nix shell nixpkgs#nodejs_22` + `npm i playwright-core`
  into a scratch dir, launch host `google-chrome-stable` with
  `chromiumSandbox:false` + `--no-sandbox`. Login via the "Test sign in" button.
- The CornerUpLeft "Replying to @user" indicator text is split across
  `<span>Replying to</span><Link>@user</Link>` — playwright `text=/Replying to
  @/i` does NOT match (separate text nodes); count cards by `hasText: 'Replying
  to'`.


## User relationships & profile lists
- `user_relationships` allows a pair to hold SEVERAL relationship rows
  (UNIQUE on follower_id+following_id+relationship_type), so follow + mute
  coexist. `GetRelationshipStatus` must read ALL rows for the pair (now returns
  `is_muted` too). Type-scoped delete (`DeleteByType`) is required for
  unfollow/unblock/unmute — the pair-wide `Delete` is used ONLY by the block flow
  (which intentionally clears everything in both directions).
- `GET /users/{username}/followers` and `/following` return flat
  `items: UserProfileResponse[]` (NOT `followers: UserWithProfile`) — the
  app-wide "paginated responses use `items`" convention. They also carry
  viewer-relative `is_following/is_blocked/is_muted` (batch-hydrated).
- `UserProfileResponse` now always serializes `is_following/is_blocked/is_muted`
  (false by default). Only the profile + followers/following endpoints hydrate
  them; search/suggested/likers/reposters do NOT — don't read those flags there.
- Mute silences notifications via `notification_service.Create` (actor muted →
  drop). It does NOT filter feeds/DMs.
- `postStore.GetUserFeed` mode variants live in `runUserFeedQuery`/
  `buildUserFeedQuery` (modes: all/replies/media) — reuse for user-feed SQL.
- **`FetchPostMedia` scans `alt_text` into a `string`**: a `NULL` alt_text row
  500s the media feed. API posts always insert `''` (never NULL); hand-written
  test inserts must set `alt_text` explicitly.

## Theme system (current work, uncommitted)
- Themes swap via CSS variables: `theme-themes.css` holds scoped blocks
  `:root[data-theme="..."]` / `.dark[data-theme="..."]` setting shadcn tokens
  (`--background/--foreground/--primary/--border/--radius` …); Tailwind v4
  `@theme inline` in `index.css` maps utilities to those vars, so setting
  `data-theme` on `<html>` (ThemeContext) live re-themes everything.
- **Tailwind v4 `@theme inline` emits `background-color: var(--background)`
  verbatim** — HSL triplets like `240 21% 15%` need `hsl()`/`oklch()` wrapping or
  they're invalid (Catppuccin "black and white" bug). Hex values are safest.
- `ThemeContext` (web/src/contexts) drives `--app-font-sans`
  (`FONT_STACKS[font]`, `--font-sans: var(--app-font-sans)` mapped in `@theme
  inline`), `--radius` (user slider), and `data-theme`. `THEME_CATALOG` marks the
  per-theme default font/radius (reset on theme switch).
- Composite `[data-slot="…"]` + `[data-theme]` selectors (specificity 0,2,0) beat
  single-class Tailwind utilities — used for scoped overrides in themes.
- Comic theme (`fun-comic`, ground truth = showcase `.comic`/`.comicdark`):
  reusable pattern for a "character" theme —
  - *Light:* cream `#fef4e0` bg, black `#111` ink, yellow `#ffd93d` shell/sidebar
    (`--sidebar`), sky-blue `#4dd2ff` primary pills, blush `#ff9e9e` muted/secondary,
    pink `#ff4d6d` destructive/ring.
  - *Dark:* purple `#161221` bg, lavender `#cfc4e8` card text, layered card purples
    `#211a33/#2d2145/#33203a`, **yellow ink `#ffd93d` for borders/inputs (black
    outlines vanish on dark)**, pink `#ff5c8a` shadows + stroke, sky-blue buttons
    with `#161221` fg.
  - Halftone dots = plain `radial-gradient(var(--comic-dot) 1.2px, transparent
    1.2px)` + `background-size: 14px` on the feed column
    `.bg-background\/25` (escaped `/`), `--comic-dot` is `#111` light /
    `rgba(255,255,255,.08)` dark. Cards stay solid so text never fights dots.
  - Inked look = `border: 3px solid var(--border)`, zero-blur hard shadows
    (`6px 6px 0 0 var(--comic-shadow)`), pill buttons (`border-radius: 9999px` +
    `5px 5px 0 0 var(--comic-btn-shadow)`), snappy `:active` 2px press.
  - Headlines = Bangers font + `-webkit-text-stroke` (2px ink light / yellow dark)
    on `h1` only; body stays Comic Neue (`FONT_STACKS['comic']`).
- The old comic halftone used a fixed `[data-halftone]` div + `filter: contrast(24)`
  + mask + rotate — **removed, replaced by the simple dot texture above**. Sliders
  for dot opacity/color were removed with it; the pattern (halftone on one surface,
  solid cards above) is the legible approach.
- Sidebar nav item colors are hardcoded `text-gray-800 dark:text-gray-100` in
  SocialMediaLayout NavItem — readable on comic's yellow/purple shell, no override.

## Dev stack & parallel agents
- The whole repo uses ONE shared Docker compose stack (`docker compose ...`). Running
  `docker compose up --build -d` from ANY worktree/branch replaces the running `api`
  and `web` containers. If two agent sessions are active at once, they silently
  overwrite each other's running build — the browser can flip between branches
  mid-session. When verifying in a browser, rebuild your own containers last and
  re-check after any other agent activity. (`web` nginx serves a built bundle on
  `:5173`; `api` on `:2021`.)
- No host Node/npm. Use `nix shell nixpkgs#nodejs_22` for host-side node, or the
  `web-tools` compose service (node:24-alpine, mounts `./web`) for `npm ci/build/lint`.
  Playwright: install the npm pkg via the nix node, then launch host `google-chrome`
  with `executablePath` + `chromiumSandbox:false` + `--no-sandbox`.

## React Query cache gotchas
- `invalidateQueries` on a query whose refetch 404s does NOT clear stale `data` —
  React Query v5 retains the last good data for active queries. If a cached entity
  can "cease to exist" (pinned post, deleted post), purge it explicitly with
  `queryClient.removeQueries(...)` in the mutation `onSuccess` (see
  `web/src/hooks/usePost.ts` `usePinPost`/`useDeletePost`).
- The bookmarks page badge counts come from the `['bookmark-categories']` query; it
  is not touched by the generic engagement mutation, so bookmark mutations must
  invalidate it explicitly or category `post_count`s go stale.

## Server: parent-chain hydration
- `postStore.GetParentChain` collects every row in the recursive CTE (including
  soft-deleted parents) then calls `GetFullPostByID` per id, which filters
  `soft_deleted = FALSE` → a deleted parent used to 500 the ancestors fetch and
  break the reply's post page. Soft-deleted chain rows must be skipped before
  hydration. The reply still carries its `parent` summary (`deleted:true`, no
  author) for the "Replying to a deleted post" UI state.


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

## Tooling / environment (Docker)
- Full stack runs via root `compose.yaml`; `make dev` = `docker compose up --build -d`.
  Frontend served by nginx (host `:5173`) which reverse-proxies `/api/*` + `/swagger/*`
  to the `api` container — the app is single-origin, so no CORS.
- No Go/Node on the host. Go tooling runs in the `tools` compose service
  (`server` mounted at `/src`); frontend tooling in `web-tools` (`web` at `/src`).
  Both are `profile: tools`, not started by `make dev`.
- `migrate` golang-migrate CLI is built into the api image and applied on start via
  `/app/migrate` (entrypoint `scripts/docker-entrypoint.sh`, POSTGRES_URL env). Manual:
  `make migrate-up` / `make new-migration <name>` (via tools container).
- **Swag resolves `models.X` from the imports of the file containing the annotation** —
  if a handler's annotations reference `models.Foo`, that file must import `models`
  (settings_handler has a `var _ models.UserSettings` for this). Regen: `make swag`
  (writes to `server/docs`).
- Tests: `make test` = `docker compose --profile tools run --rm tools go test ./...`.
  Uses a throwaway DB `social_test` (see `internal/testutil`), create/drop against
  the `db` container via `TEST_DB_ADDRESS=db:5432` (configured in compose).
- Seed: `make seed` runs `/app/seed` in the api image; idempotent (exits early if
  `alice@example.com` exists).
- Docker on NixOS must be enabled (`virtualisation.docker.enable = true`) in
  `~/nixos-config`. Kernel has no legacy iptables + nftables is on; if published
  ports fail, switch Docker to the nftables driver (see comment in configuration.nix).
## Frontend
- `npm run build` needs esbuild/tailwind oxide install scripts approved
  (`npm approve-scripts esbuild @tailwindcss/oxide`); `web/Dockerfile` uses `npm ci`
  which honors package.json `allowScripts`. react-day-picker must be v9 for
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
- Frontend API base URL is relative (`/api/v1`, `VITE_API_BASE_URL`); `npm run dev`
  still works locally thanks to the vite dev proxy to `localhost:2021` (vite.config.ts).
- Frontend build/lint (Docker): `docker compose --profile tools run --rm web-tools npm run build`.
- Phase 1 notifications: migration `000009` creates persistent notifications; migration
  `000010` changes post references to `ON DELETE SET NULL` so history survives deletion.
  `/api/v1/stream` is cookie-authenticated SSE; nginx disables buffering for that route.
  The in-process realtime hub emits `notification.new`, `feed.post_created`, and
  `stream.resync`; frontend EventSource invalidates the relevant React Query caches.
- Phase 2 search: migration `000011` creates `hashtags`/`post_hashtags` and a GIN
  full-text index. Hashtags are normalized lowercase at write time; post search,
  user search, hashtag feeds, and 24-hour top-level-post trends are protected routes.
- Phase 3 post power: migration `000012` adds `posts.edited_at`/`is_pinned`,
  `post_edits`, `polls`/`poll_options`/`poll_votes`, and a partial unique index
  `posts_one_pinned_per_author_idx (author_id) WHERE is_pinned AND NOT soft_deleted`
  (one pin per author). Polls are top-level-only, 2-4 options, one vote/user.
- **Poll votes on a duplicate return 409 via `23505` — the app driver is `pgx/v5`, so
  the unique-violation check must use `*pgconn.PgError` (`github.com/jackc/pgx/v5/pgconn`),
  NOT `*pq.Error` from `lib/pq`** (real bug found by review test). `pq.Array()` still
  works as a `driver.Valuer` for `ANY($1)` params even under pgx.
- **Polls are hydrated in every feed path** via batch `store.Polls.GetForPosts(ids)` +
  service `hydratePolls`, called alongside `hydrateEngagement`. Feeds dropped polls
  before this (critical review finding). `GetFullPostByID`, `GetPinned`, search feed all
  use the same batch helper now.
- Poll/option IDs are global SERIALs (NOT per-post), so `option_id` values are sparse
  across posts — never hardcode `1`; use the ids returned by the API (frontend does).
- `GetByID` (used by `SetPostContextMiddleware`) does NOT filter `soft_deleted`; the
  middleware itself returns 404 for soft-deleted posts so ALL `{postID}` sub-routes
  (edits, poll votes, likes) correctly 404 after deletion — don't add a filter to
  `GetByID` itself, some internals rely on reading deleted rows.
- Swagger: new handler annotations need `@Router` + matching `@Security ApiKeyAuth`
  or the endpoint won't appear in `server/docs` after `make swag`.
- Edit flow: `PostService.Update` is a no-op when content unchanged (no history row);
  `Store.Posts.Update` guards `soft_deleted=FALSE`, `PostService` enforces ownership.
- Phase 4 badges: migration `000013` adds `users.is_admin`, `badges` (catalog with
  `kind earned|assigned` + `criteria` JSONB), and `user_badges` (admin grants only).
  Earned badges are COMPUTED on read, never stored: `badgeStore.getMetrics` batches
  account age / top-level post count / followers / likes-received, then
  `GetBadgesForUsers` merges earned + assigned. Hydration happens in handlers via
  `service.Badges.HydrateProfiles` / `HydrateUserWithProfiles` — every profile path
  (single user, search, followers/following, likers/reposters) must remember to
  hydrate or badges silently disappear.
- `UserProfileResponse` carries an internal `UserID json:"-"` so flat-profile
  responses can be batch-badged without a username→id lookup. Keep it populated
  when constructing responses in stores.
- `users.is_admin` must be selected in every query scanning into `models.User`
  (GetByID/GetByEmail/GetByUsername/GetUserProfileByUsername/followers/following).
  `admin_handler` routes live under `/admin` behind `AdminOnlyMiddleware`, which
  must be mounted AFTER `AuthTokenMiddleware`.
- Badge icons are stored as lucide-react component names; the `UserBadges` component
  falls back to `Award` when a name no longer exists. `deleteBadge` refuses (409)
  earned badges and assigned badges still held by a user.
- Phase 5 explore+lists: migration `000014` adds `lists` and `list_members`
  (composite PK). `postStore.GetListFeed` is a clone of `GetHomeFeed`
  (`INNER JOIN list_members lm ON p.author_id = lm.user_id`, top-level only,
  same keyset cursor). Suggested users = user search query minus the ILIKE
  filter, excluding self/followed/blocked via `NOT EXISTS` on `user_relationships`.
- The hydrate helpers (`hydrateEngagement`, `hydratePolls`) are now package-level
  funcs in post_service.go taking `(ctx, *store.Store, ...)` so both PostService and
  ListService share them — keep that signature when adding feed consumers.
- List responses include `owner_username` (via `JOIN users`) so the frontend can
  gate owner-only UI; `GET /users/{username}/lists` lists a user's public lists.
- The sidebar "Who to follow" and ExplorePage both call `GET /users/suggested`
  (`limit` default 5, max 20 for sidebar; 20 for the page).
- Phase 6 DMs: migration `000015` adds `conversations` (canonical
  `participant_a < participant_b`, `UNIQUE(a,b)`) and `messages` (`read_at`
  partial index `messages_unread_idx (conversation_id, sender_id) WHERE read_at IS NULL`).
- **chi route conflict pitfall:** you cannot register both `POST /dms/{username}`
  and a sibling `Route("/{conversationID}", ...)` with *different* parameter
  names at the same level — it shadows/misfires (405). Nest the second group
  under a literal segment instead: `/dms/conversations/{conversationID}/*`.
- `GET /dms/conversations/{id}` (single conversation) must take `viewerID` so the
  store can attach `other_participant` — the raw pair doesn't tell the client who
  to show. `ListConversations` uses a `LEFT JOIN LATERAL` for last-message.
- DM send publishes `dm.new` + `dm.unread` to the recipient AND `dm.unread` to
  the sender (their per-conversation read state changed) after DB writes. Frontend
  invalidates `dm-conversations`/`dm-unread-count` on `dm.new`, `dm.unread`, and
  `stream.resync` in NotificationsContext. Message query cache keys are
  `dm-messages`/`dm-conversation`.
- **Search/hashtag feeds must default `Engagement` to an empty struct** —
  `search_service.hydrateFeed` assigns `item.Engagement = engagements[item.ID]`
  which is nil for posts with no likes/reposts/bookmarks; the frontend FeedPost
  dereferences `engagement.*` and crashes (blank screen). Always populate counts
  (`LikeCount = item.LikesCount`, etc.) like `hydrateEngagement` in post_service.
- Frontend engagement optimistic updates are **delta-based**: `applyEngagementMerge`
  in `usePost.ts` adds numeric fields (`like_count`/`repost_count`/`bookmark_count`)
  to the current value (clamped ≥0) instead of overwriting, so like→unlike at 0
  stays 0 rather than −1. Booleans (`is_*`) are set absolutely.
- Hashtag rendering is centralized in `web/src/components/HashtagText.tsx`
  (accent-colored `text-blue-600 dark:text-blue-400` links). ComposeContent shows
  a live highlight via a mirror `<div>` behind a transparent-caret textarea; the
  `.hashtag-composer` CSS in `index.css` keeps the placeholder visible while the
  text fill is transparent.
