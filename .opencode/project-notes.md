# Admin metrics dashboard (agent/admin-metrics-panel)
- Host stats (`internal/metrics/host.go`) read `/proc/stat|meminfo|loadavg|uptime`
  inside the api container — values are host-wide because containers share the
  kernel. Disk needs the `- /:/host:ro` bind mount (compose.yaml) or falls back
  to the container root. CPU% is a 200ms two-sample delta.
- `page_views` (migration 000022) is written by `VisitMiddleware` on EVERY
  protected GET (mounted after auth, so user_id is set); `/api/v1/admin/*` is
  excluded so the dashboard's 5s poll doesn't self-inflate the table. "Visits"
  == authenticated GET traffic (the SPA hides everything behind login).
- New admin store pattern: `Store.Metrics`/`Service.Metrics` (metrics_store.go)
  follows the sub-store wiring (store.go interface + NewStore + service.go
  interface + NewService). If adding another aggregate query, mirror `AppStats`.
- **Host history sampler**: migration 000023 `host_metrics_samples` is written by
  `internal/metrics/sampler.go` (started in cmd/api/main.go, first sample
  immediately then every 60s, hourly prune). Sampling is process-lifetime, NOT
  request-driven — history accrues 24/7. Retention (both page_views and host
  samples) = `METRICS_RETENTION_DAYS`, default 90.
- `GET /admin/metrics/history?range=24h|7d|30d&days=1-90`: host series is
  downsampled server-side with `date_bin()` in metrics_store.go into fixed
  buckets (24h → 1-min, 7d → 5-min, 30d → 15-min; AVG per bucket) so long
  ranges don't ship thousands of points to the browser. `days` is independent of
  `range` and controls the views series. Frontend: dashboard has a separate
  14/30/90-day toggle for the views bar chart; views come from *history*, not the
  live snapshot's `by_day`.
- shadcn `ui/chart.tsx` (`ChartContainer`, `ChartTooltipContent`, `ChartLegend`)
  was previously unused across src; recharts ^2.15 + React 19. To theme lines:
  put `color: "var(--chart-N)"` (note: hyphens, NOT `--color-chart-N`) in the
  chart `config`, then `stroke="var(--color-<key>)"` on the `<Line/>`/`<Bar/>` —
  ChartStyle emits `--color-<key>` scoped to the chart wrapper. Use
  `className="h-64 aspect-auto"` on ChartContainer to override its default
  `aspect-video`.

---
# Snappy UX: optimistic DMs, staleTime, prefetch (agent/snappy-ux)
- **`onMutate` is NOT a valid `mutate()` option** — `MutateOptions` only supports
  onSuccess/onError/onSettled. To do optimistic UI on a mutation, either put it
  in the `useMutation({ onMutate })` definition (hook owns it) or, for page-local
  state like clearing the composer, do it BEFORE calling `mutate()` and restore in
  `onError`. `useSendMessage` does both: the hook now has an `onMutate` cache
  write, and the page clears the input pre-mutate.
- Optimistic message pattern (`useSendMessage`): cancel `['dm-messages', id]`,
  snapshot, prepend a `pending:true` temp (negative id) into `page[0].items`
  (messages are newest-first), return `{previous, conversationId, tempId}`; on
  success `setQueryData` swaps temp id → `_data.data`, onError restores
  `previous`. `Message.pending?` drives the "Sending…" label + `opacity-70`.
- **Mutation variables must carry the sender** for optimistic rendering — the DM
  mutation takes `{ username, body, conversationId?, sender }`; sender comes from
  `UserContext` (`displayName`→`display_name`, `profilePictureUUID`→
  `profile_picture_uuid`).
- QueryClient now has a global default `staleTime: 60_000` (was 0 → every mount
  refetched). Feed keeps its explicit `staleTime: Infinity`. Prefetching an
  infinite query must use `queryClient.prefetchInfiniteQuery(options)` (NOT
  `prefetchQuery`) or the cache gets the wrong `InfiniteData` shape and the page
  crashes. Shared options live in `web/src/hooks/useNotifications.ts`.
- SSE `dm.new`/`dm.unread` in NotificationsContext now also invalidates
  `['dm-messages']` so live incoming messages appear while viewing a conversation.
---
## Birth date "not set" sentinel (agent/nickname-default follow-up)
- The API serializes an unset `birth_date` (DB NULL) as `"0001-01-01"` (Go zero `models.Date` → `MarshalJSON` always emits). Frontend must treat that sentinel as "no birthday" — ProfilePage `formatDate`/editor use `UNSET_BIRTH_DATE = "0001-01-01"` via `isUnsetBirthDate()`. Decision: wire format stays as-is; the client hides/clears it. (If the server ever switches to `null`, the `!dateString` branch already covers it.)
- `models.Date.Scope` note: `Date.Value()` returns the zero `time.Time` (not nil), so a PATCH `birth_date: ""`/zero persists "0001-01-01" in the DB instead of NULL — a user can't truly clear a birthday. Known, deliberately unfixed (legacy/edge; dev data gets wiped).

## Profile display_name default (agent/nickname-default)
- `user_profiles.display_name` is `NOT NULL` with NO column default (migration 000004) — the value comes solely from the store INSERT, which used to hardcode `''`. New profiles now default `display_name = username` (`user_store.go` `Users.Create`). If you create profiles anywhere new, pass a real display name or rely on `$1`/`user.Username` — a bare INSERT leaves the account "naked".
- Username (max 16) always fits the 50-char display_name column, so the username default can't overflow.
- A display-name backfill migration (`000022`) was drafted then dropped — existing accounts are being wiped; only the creation default ships. No migration on this branch.

---
## Page logo = grok_cutout (agent/grok-logo)
- New logo master is `grok_cutout.svg`/`.png` in
  `/home/bau/Programming/svg-img/new-stuff-here/` (a background-transparent
  cutout; SVG = 1024² vector, PNG = 4096² raster — same art).
- The art is NOT centered: opaque content flushes to the bottom-right corner
  (SVG bbox `793x931+231+93`; render corner pixel is opaque). Regenerate with
  trim + recenter, or the `rounded-full` 40px sidebar circle crop clips it.
- Recipe (matches the old goose fill ratio — max reach ≈ 1.045× the inscribed
  circle radius): `rsvg-convert -w 1024` the SVG → `magick -trim +repage` →
  `-resize 694x815` → `-gravity center -background none -extent 1024x1024` →
  downscale to 80×80 for the sidebar and to 16/32/48/64 → `magick` ICO.
- Web files keep the old names (`/favicon.ico`, `/gaggle-goose.png`) so no
  code changes were needed — only the two binary assets swapped.
---
## F5 boot splash (agent/f5-loading-ui)
- Every layout page gated on `token === undefined` rendered bare `Loading...`
  text (`SocialMediaLayout.tsx`) — now a branded `AppSplash`, plus a pre-JS
  inline splash in `index.html` inside `#root`.
- React `createRoot` overwrites `#root` children on first render, so pre-JS
  splash markup in `index.html` gets replaced cleanly at mount.
- **Axios has NO default timeout** (timeout 0 = wait forever). The auth
  bootstrap now passes `AbortSignal.timeout(10_000)` to the refresh-token
  POST and `/users/me` GET so a hung AWS box can't park the app on the boot
  screen indefinitely; on abort it falls back to signed-out (login redirect).
  Tradeoff: >10s refresh treats the session as gone.
---
## Messages: /messages/new + search debounce
- **Static routes have NO route params**: `useParams()` on route `/messages/new` returns `{}` — `conversationId` is `undefined`, not `"new"`. ConversationPage tested `conversationIdStr === "new"`, so `/messages/new` fell through to the existing-conversation branch, computed `Number(undefined)` = NaN, disabled the dm-conversation query, and rendered "Conversation not found." Fix pattern: `const isNew = !conversationIdStr || conversationIdStr === "new"` (ConversationPage.tsx:21).
- `NewMessageComposer` (MessagesPage.tsx) passed the live query to `useSearchUsers` → one `GET /search?type=users` per keystroke. Now debounces via `useDebounce` and the shared `SEARCH_DEBOUNCE_MS = 300` (exported from `web/src/hooks/useDebounce.ts`). Other keystroke-fired searches were the same way and got fixed too: ListPage `MemberSearch` (add-user), ExplorePage live `useSearchPosts`. The constant is the single source of truth — tune debounce there, don't inline `300` at new call sites. (FeedPost's 150 ms is a client-side category filter, not an API call; stays as-is.)
- Message threads are ALREADY fixed-height + internal-scroll (flex-1 min-h-0 overflow-y-auto under the h-screen column). The remaining ~98px page scroll on message pages is the app-wide sidebar (taller than 100vh on typical viewports), present on every page and unrelated to messages.
- Frontend has no test runner (no vitest) — browser verification via playwright-core + host google-chrome + a local `nix shell nixpkgs#nodejs_22` vite dev on a non-standard port works without touching the shared compose stack.
---
## Login lab + useLoginFlow (agent/login-experiments)
- **Do NOT render a decorative heading via `FormLabel`.** The step-flow
  password step put "Welcome back" in a `FormLabel`, which carries
  `data-error` + `data-[error=true]:text-destructive` (`ui/form.tsx:98`) —
  so a field error painted the H1/title red along with the error text.
  If text is a heading, render a real heading; `FormLabel` is for the
  actual field label.
- **studio-claude light theme had `--destructive: oklch(0.19 0 106.59)`** =
  neutral dark gray (identical to `--card-foreground`) → every
  `text-destructive` error/destructive UI looked like plain text on the
  default theme. Fixed to `oklch(0.577 0.245 27.325)`. Its `.dark` block
  (line 61) was already red.
- **react-hook-form `handleSubmit` only calls its `onValid` when the WHOLE
  form is valid.** A multi-step flow whose step 1 only needs `identifier`
  can't gate step-advance inside `handleSubmit` — the empty step-2
  `password` keeps the form globally invalid, so advancing never fires
  (observed at runtime in `StepFlow.tsx`). Fix: raw
  `onSubmit={(e) => { e.preventDefault(); ... }}`, call
  `form.trigger('identifier')` directly to advance, and only resort to
  `form.handleSubmit(onSubmit)()` on the final step.
- Login auth flow (zod schema, `useLoginMutation`, `setUser`, navigate '/')
  is shared via `web/src/hooks/useLoginFlow.ts` — used by `LoginPage` AND
  every `login-lab` variant so the real page and experiments can't drift.
- Glassmorphism variant needs the `login-lab-drift` keyframes added to
  `web/src/index.css` (`@layer utilities`) — remove if the variant goes.

## Enable HTTPS (agent/enable-https)
- Compose list merge (ports) is **append**, not replace — a prod override that
  re-declares `ports` dups the dev mappings. `ports: !reset []` DOES work as a
  standalone key (db/redis) but a YAML mapping can't hold `ports: !reset []`
  AND `ports: [...]` (duplicate-key parse error), and `- !reset []` as a list
  element silently no-ops. Cleanest: drive host ports via `.env` interpolation
  in the BASE file (`WEB_PORT`, `WEB_HTTPS_PORT`) and let the prod override set
  the env values.
- certbot's `live/gaggle` is a **symlink into `archive/gaggle`** — any shared
  volume must mount the WHOLE `/etc/letsencrypt` at the same path in web +
  certbot or nginx can't follow the symlink.
- First-issuance gotcha: if the web entrypoint seeds a self-signed fallback
  and certbot runs `--keep-until-expiring`, the fresh 10yr fallback blocks
  real issuance forever. First certbot run must use `--force-renewal` (guard:
  check `[ -L live/gaggle ]` first).
- The web image ENTRYPOINT is now custom (web/docker-entrypoint.sh: self-signed
  gen then `exec nginx`); the stock `/docker-entrypoint.d/` envsubst
  templating does NOT run — nginx conf must stay static (no `$VAR` baked by the
  image).
- `COOKIE_SECURE=true` in prod compose means the refresh cookie fails over
  plain `http://<ip>` (browser rejects Secure cookies on HTTP) — expected,
  HTTPS is the migration target.

## login-experiments → /login promotion (agent/login-experiments)
- The keeper favorite (simple step flow, `StepFlow.tsx`) became the real
  login page: `LoginPage.tsx` now renders `<StepFlow footer={...}/>` with the
  footer carrying **Test sign in**, **Forgot your password?** (toggles to the
  reset card), and **Sign up** link. `StepFlow` got an optional
  `footer?: ReactNode` prop (rendered after `</Form>`, inside the form column).
- Lab variants at `/login-lab` are untouched — StepFlow there gets no footer
  (prop defaults to undefined), so experimenting in the lab won't break
  `/login`. Note the coupling: editing the StepFlow variant changes `/login`.
- `LoginPage.tsx` uses `h-screen overflow-y-auto` (mirrors the lab pane);
  `getByRole('button', {name:'Sign in'})` is ambiguous with "Test sign in"
  — playwright needs `exact: true`.
---
## Goose branding assets (agent/gaggle-goose-branding)
- `web/` had NO `public/` dir and `index.html` referenced a nonexistent
  `/vite.svg` → the favicon 404'd. This branch now ships
  `web/public/favicon.ico` (transparent goose, 16/32/48/64 frames, 32KB) +
  `web/public/gaggle-goose.png` (80×80, 6KB) — generated from the master
  `goose_max.svg` via `nix develop /home/bau/Programming/svg-img` + `rsvg-convert`
  + `magick`. Masters (1024×1024 VTracer traces, ~3MB) stay in that folder.
- Regenerate: `rsvg-convert -w N -h N goose_max.svg -o icon-N.png` then
  `magick icon-16.png icon-32.png icon-48.png icon-64.png favicon.ico`.
- Branding "G" logo lived solely in `web/src/layout/SocialMediaLayout.tsx`
  (App Logo block); no other file rendered it.
- Beware: `glob`/search tools in this setup skip hidden dirs — always check
  `.opencode/` explicitly (writing project-notes.md as "new" overwrote the
  existing shared log; restore via `git checkout HEAD -- .opencode/project-notes.md`).

## Auth-validation sweep #1–#6 (agent/auth-validation-consistency)
- DB column limits (posts.content 280, polls.question 140, options 100) were NOT enforced in the
  API — over-long payloads 500'd via Postgres. Fix: rune-aware checks in `post_service.go`
  (`validateContentLength` + `validatePoll`), constants `maxPostContentLength=280` /
  `maxPollQuestionLength=140`. Rune-aware because Postgres counts characters, JS `.length` counts
  UTF-16 units — the API check is the hard stop, the frontend `maxLength` just UX.
- go-playground/validator has NO built-in `regexp` tag — using `regexp=...` panics at first
  validation unless registered. Added a `regexp` custom validator in `util/json.go` init.
- Profile `UserProfile` model: Bio had `required`, Location/Website had `min=3` — but the DB
  defaults those to `''`, so empty/short clears 400'd. Pattern to remember: validation tags must
  be compatible with actual DB column defaults/NULL semantics.
- `models.Date` `UnmarshalJSON` rejects `""`; the profile form sends `birth_date: ""` for users
  without one → 400 "invalid request payload". Date now maps `""` → zero time (matches DB NULL
  round-trip; zero Time marshals as "0001-01-01").
- Username length rules live in FOUR places and were inconsistent: `RegisterRequest.Username`
  (server) + `SignupPage.tsx` zod = min 3, `LoginPage.tsx` zod was min 4 (bug — a 3-char
  user couldn't sign in), `LoginRequest.Identifier` had no min. Aligned everything to min 3.
- Duplicate username/email register errors were already specific on the backend
  (`USERNAME_EXISTS`/`EMAIL_EXISTS`, matched by index name in `user_store.go.140-153`), but
  `SignupPage.tsx`'s catch swallowed the axios error. Frontend error surfacing pattern:
  `(err as AxiosError<Envelope<unknown>>)?.response?.data?.error?.message` (see AuthContext).
- Login stays intentionally generic ("invalid credentials") — don't leak account existence there.

## Seed backdating: created_at not settable through the store
- `post_store.go:260` (`Create`) and `:288` (`CreateQuotedPost`) INSERTs omit
  `created_at` (DB default `CURRENT_TIMESTAMP`), so a seed can't backdate
  posts. **Fix (chosen in seed-data-strategy spec)**: add `created_at, updated_at`
  to both INSERT columns and pass `COALESCE($6, CURRENT_TIMESTAMP)`, passing
  `nil` when `post.CreatedAt.IsZero()`. App call sites (`post_service.go:467`,
  `:1031`) build fresh posts with zero `CreatedAt` → `NULL` → `now()`, so
  behavior is unchanged; `RETURNING` already fills `post.CreatedAt`.
- `models.Post` already has `CreatedAt time.Time` (post.go:42) — no model change.

## GitHub / git on NixOS
- `~/.config/git/config` is a read-only home-manager symlink into the nix
  store. Anything writing the *global* git config fails ("could not lock config
  file ... Read-only file system") — e.g. the final `git config --global`
  step of `gh auth login`. Workaround: push over SSH (`id_ed25519` registered
  on GitHub), not gh's HTTPS credential helper. Repo-level `git remote`/push
  config is unaffected (`.git/config` is writable).
- gh auth login's default OAuth scopes (`repo`, `workflow`, etc.) DO NOT include
  `admin:public_key`, so `gh ssh-key add` returns HTTP 404 until you run
  `gh auth refresh -h github.com -s admin:public_key`. Adding a key via the
  web UI (Settings → SSH keys, type "Authentication key") sidesteps the refresh.
- Repo `main` was pushed to GitHub as `ba-reynolds/gaggle` over SSH
  (origin = git@github.com:...); home-manager sets `init.defaultBranch=master`
  but existing repos keep their own branch.

## Sidebar/mobile nav tiers (agent/sidebar-mobile-nav)
- Three responsive nav tiers: below `md` = fixed bottom nav is the ONLY nav
  (left sidebar `hidden md:block`), `md`–`lg` = icon-only sidebar rail
  (labels `hidden lg:inline`), `lg+` = full sidebar labels + right rail.
  Grid keeps 12 cols: base main `col-span-12`, md 2+10, lg 2+7+3.
- Icon-only mode centers content: `NavItem` uses `justify-center lg:justify-start gap-x-4`
  instead of left-aligned `space-x-4`, Post button becomes a `md:w-14 md:h-14`
  circle with a `PenSquare` icon (returns to full pill + "Post" at `lg+`).
- Below `md` the account dropdown (Log out, and Lists/Mentions/Admin links)
  is NOT reachable — sidebar is hidden; bottom nav now carries Explore + DMs
  (with unread badge), but logout still only lives in the sidebar dropdown.

## Theme merge + kanagawa light + radius removal (agent/trim-theme-catalog)
- Catppuccin catalog is now ONE flavor (mocha), 3 entries: `catppuccin-mocha-mauve` /
  `blue` / `peach`. Macchiato/frappe ids removed from `THEME_CATALOG` AND their
  CSS blocks deleted from `theme-themes.css` — a stored macchiato/frappe id
  falls back to Claude via `findTheme`.
- Radius is theme-owned now: `setRadius`/`radius`/`vite-ui-radius` gone from
  `ThemeContext.tsx`. On themeId change the provider sets `--radius` from
  `definition.defaultRadius`. The per-theme `--radius: 0.5rem/0.625rem/0rem`
  lines left in `theme-themes.css` are dead weight (inline style wins).
- Kanagawa light ≠ dark: `:root[data-theme="icon-kanagawa"]` = washi light
  palette, `.dark[...]` = wave dark palette. The old
  `:root[data-theme="icon-kanagawa"]:not(.dark) .text-gray-800` override is
  deleted — pointless now that the light sidebar is light.
- On this NixOS box `npm` isn't on PATH: run frontend commands via
  `nix shell nixpkgs#nodejs --command npm ...` (works with the checked-in
  node_modules-free worktree; `npm ci` first).

## Profile page action buttons (agent/profile-action-buttons-align)
- ProfilePage's action row (`<div class="flex justify-end …">`) used to ALWAYS
  render "Edit profile" and hide it with `visible/invisible` — `visibility:hidden`
  keeps layout space, so on other users' profiles the invisible ~106px button
  reserved the rightmost slot and the visible Unfollow/Message/… stopped
  ~114px short of the right edge (reads as "centered"; overflowed left at ~375px).
  Fixed: conditionally render `{isCurrentUser && <Button>Edit profile` instead of
  the invisible placeholder.

## Privacy / visibility enforcement (account-and-post-privacy)
- `posts.visibility` (`public|followers|mentions`) + `posts.mentioned_user_ids int[]`
  (resolved at create) and `users.is_private` (synced from settings
  `profileVisibility`; `private` AND `friends` both mean followers-only).
- ALL enforcement is centralized in `service/filterVisiblePosts(ctx, st, posts, viewerID)`
  (package-level, like the hydrate helpers) — one batched `GetRelationshipStatuses`
  + one `Users.GetIsPrivate` per unique author. Single-post reads gate via the
  same filter in `PostService.GetFullPostByID`/`GetPinned`; engagement writes via
  `CanViewPost`. **Any new feed consumer must call `filterVisiblePosts` or it leaks.**
- **`pq.Array(&[]int)` cannot SCAN a postgres int[]** — only `[]int64`. Use the
  `scanMentionedIDs` adapter in post_store.go. (VALUER side `pq.Array([]int)` is fine.)
- `mentioned_user_ids` is `NOT NULL` — `pq.Array(nil)` sends NULL and violates it;
  always pass `pq.Array(nonNilIntSlice(...))`.
- Media files under `GET /media/{uuid}` stay public by design (unguessable UUIDs,
  `<img>` can't send auth headers) — a followers/mentions-only post's media is
  reachable if you know the UUID. Known limitation.
- Migration `000017` backfills `users.is_private` from existing settings JSONB.

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
