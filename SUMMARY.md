# SUMMARY — per-worktree isolated preview stacks

Lets each agent worktree run its OWN copy of the whole stack (db + redis + api
+ web) on private ports and its own volumes, so you can connect to one agent's
build without clobbering the shared dev stack or other agents' previews.

## What was changed and why

- Agents used to be forced through ONE shared compose stack: `make dev` from
  any worktree replaced the same `gaggle-*` containers and shared Postgres
  data, so you couldn't tell which build you were looking at.
- **Per-worktree preview (the fix).** `compose.yaml` stopped hard-coding
  `container_name` on every service and the API's `2021:2021` port (now
  `${API_PORT:-2021}`) so docker compose `-p <project>` can namespace a whole
  stack per branch. Caches (`go_mod_cache`, `npm_cache`) got explicit names so
  branches share build caches while DB/redis/media data stays isolated.
- `scripts/proj-up.sh` + `make proj-dev` (run INSIDE `agent-branch/<slug>/`)
  derive a project `gaggle-<slug>`, allocate hash-based ports persisted in
  `~/.local/state/gaggle-proj/<project>.env` (stable across re-runs), and start
  the stack with `docker compose -p ... up --build -d --wait`, printing
  `Frontend: http://localhost:<port>`. `proj-stop/proj-logs/proj-ps/proj-seed`
  manage it.
- **Agent loop:** the project's `.opencode/commands/new-task.md` (a
  project-scoped override of the global one) tells agents to stand up their own
  preview and report the frontend port back; `.opencode/commands/status.md`
  lists preview URLs; `.opencode/commands/merge-all.md` tears previews down on
  merge.
- Verification: `docker compose -p` dry-ran on a copy, Makefile targets
  reached, `proj-up.sh` logic exercised.

---
# SUMMARY — admin-metrics-panel

Adds a live metrics dashboard to the existing `/admin` area so an admin can
see EC2 instance stats (CPU, memory, load, uptime, disk), platform counters,
active users, and visit traffic at a glance.

## What was changed and why

- **Backend — host stats** (`server/internal/metrics/host.go`, new pkg). Reads
  whole-box CPU %, memory, load average, uptime and disk usage straight from
  `/proc/*` + `statfs`. The api container shares the host kernel, so
  `/proc/stat`, `/proc/meminfo`, `/proc/loadavg`, `/proc/uptime` report the
  host, not the container — no cloud creds or extra agents. CPU% is a 200ms
  busy/total delta sample. Disk stats statfs the host filesystem mounted
  read-only at `/host` (new `- /:/host:ro` bind mount in `compose.yaml`), with
  a `/` fallback when the mount is absent (tests).
- **Backend — visit tracking** (migration `000022_create-page-views`).
  New `page_views` table (`user_id` FK SET NULL, `ip INET`, `method`, `path`,
  `status`, `created_at`; indexes on `created_at DESC` + `user_id`).
  `server/internal/middleware/visit.go` records GETs served inside the
  protected API tree (mounted in `router.go` right after `AuthTokenMiddleware`,
  so the user id is in context). Excludes non-GETs and everything under
  `/api/v1/admin/*` so the dashboard's own 5s poll never pollutes the table.
  The existing `clientIP()` (rate_limit.go, X-Forwarded-For aware) is reused.
- **Backend — metrics store + endpoint.** `store/metrics_store.go` exposes
  `Record` + aggregate queries (app counters, views-by-day for the last 14
  days, views in the last 60s, distinct active users since a timestamp), wired
  through `Store` and `Service` like every other sub-store. New
  `GET /admin/metrics` (admin-only, swagger-annotated) returns one
  `AdminMetrics` snapshot `{host, app, active, views}`.
- **Frontend.** `/admin` is now tabbed: **Overview** (new `MetricsDashboard`
  component: meter cards for CPU/mem/disk, load + uptime, platform stat cards,
  and a pure-CSS views-per-day bar chart — no chart lib) and **Badges** (the
  existing badge-management UI, untouched). Polls the metrics endpoint every 5s
  via `useAdminMetrics` (`refetchInterval`, `web/src/hooks/useAdmin.ts`).
  `web/src/api/admin.ts` + `web/src/types/api.ts` gained the endpoint + types.
- **Tests.** `TestAdminMetrics` in `server/internal/handlers/integration_test.go`:
  403 for non-admins, 200 with populated host/app/active/views for admins, the
  visit middleware records a `GET /users/me` exactly once, never records
  `/admin/metrics` (feedback loop) nor POSTs. Full `go test ./...` + frontend
  `npm run build` + `npm run lint` (0 errors) pass.

## Files touched

- `server/cmd/migrate/migrations/000022_create-page-views.{up,down}.sql` (new)
- `server/internal/metrics/host.go` (new)
- `server/internal/middleware/visit.go` (new)
- `server/internal/store/metrics_store.go` (new)
- `server/internal/models/metrics.go` (new)
- `server/internal/store/store.go`, `server/internal/service/service.go` (wire
  Metrics sub-store)
- `server/internal/handlers/admin_handler.go` (Metrics endpoint + swagger)
- `server/internal/api/router.go` (visit middleware + `/admin/metrics` route)
- `server/internal/handlers/integration_test.go` (TestAdminMetrics)
- `server/docs/*` (regenerated swagger)
- `compose.yaml` (`- /:/host:ro` api volume)
- `web/src/components/MetricsDashboard.tsx` (new)
- `web/src/pages/AdminPage.tsx` (tabs), `web/src/hooks/useAdmin.ts`,
  `web/src/api/admin.ts`, `web/src/types/api.ts`

## Follow-up — history charts (host + views)

Adds time-series history to the metrics dashboard so an admin can see how CPU /
memory / disk and visit traffic evolved over the last hours or weeks, not just
the current snapshot.

- **Backend — sampler** (`server/internal/metrics/sampler.go`, new). A
  process-lifetime goroutine started in `cmd/api/main.go` that records
  `ReadHostStats()` immediately, then every 60s (`METRICS_HOST_SAMPLE_SECONDS`),
  and prunes old rows hourly. Sampling runs whether or not anyone is on the
  dashboard, so history accrues 24/7 from deploy. Retention for BOTH
  `page_views` and host samples is `METRICS_RETENTION_DAYS` (default 90).
- **Backend — storage** (migration `000023_create-host-metrics-samples`).
  New `host_metrics_samples` table mirroring `HostStats` + `created_at`.
  `metrics_store.go` gained `InsertHostSample`, `PruneHostSamples` (and
  `PrunePageViews`, which replaced the old hourly cron), plus `HostSeries`:
  server-side downsampling via `date_bin()` into fixed buckets (24h → 1-min,
  7d → 5-min, 30d → 15-min; AVG per bucket) so long ranges never ship
  thousands of points to the browser.
- **Backend — endpoint** `GET /admin/metrics/history` (admin-only, swagger).
  Query params are independent: `range=24h|7d|30d` picks the host series width,
  `days=1-90` (default = range width) the views window. Returns
  `{range, days, host:[{ts,cpu_percent,mem_percent,disk_percent,load1}], views}`.
- **Frontend.** `MetricsDashboard` got two recharts charts, both served by the
  new history endpoint and refetched every 60s (`useAdminMetricsHistory`):
  a host-usage multi-line chart (CPU/mem/disk %, 24h/7d/30d toggle) and a
  views-per-day bar chart (14/30/90-day toggle). This is the first real use of
  `ui/chart.tsx` (`ChartContainer` + shadcn theming); the old pure-CSS
  "Views — last 14 days" bars were removed. Live "now" cards still poll every
  5s and are unchanged.
- **Tests.** `TestAdminMetricsHistory`: 403 for non-admins, 400 for bad
  `range`/`days`, 200 with a downsampled host point; verifies the `days`
  override is honored; ages rows past retention and asserts
  `PruneHostSamples`/`PrunePageViews` delete them. Full `go test ./...` +
  `go vet ./...` + `npm run build` + `npm run lint` pass.

## Files touched (follow-up)

- `server/cmd/migrate/migrations/000023_create-host-metrics-samples.{up,down}.sql` (new)
- `server/internal/metrics/sampler.go` (new)
- `server/internal/store/metrics_store.go` (history + prune queries)
- `server/internal/models/metrics.go` (`MetricsHistory`, `HostSamplePoint`, ranges)
- `server/internal/handlers/admin_handler.go` (History endpoint + swagger)
- `server/internal/api/router.go` (route), `server/internal/service/service.go` (wiring)
- `server/cmd/api/main.go` (start sampler), `server/pkg/config/config.go` (envs)
- `server/internal/handlers/integration_test.go` (TestAdminMetricsHistory)
- `server/docs/*` (regenerated swagger)
- `web/src/components/MetricsDashboard.tsx`, `web/src/hooks/useAdmin.ts`,
  `web/src/api/admin.ts`, `web/src/types/api.ts`

## Reviewer checkpoints

- **"Visits" semantic**: since the app is an SPA, page views are proxied by
  authenticated GET traffic on API endpoints (everything meaningful is behind
  login). The `/admin/metrics` poll is excluded so it doesn't self-inflate.
- **Host stats accuracy**: inside Docker the values are host-wide because
  `/proc` is shared; the disk number depends on the `/host` bind mount being
  present (prod compose merges it in via the base file).
- **DB write per GET**: every protected GET now INSERTs into `page_views`
  (best-effort, non-fatal on error). Fine at this app's traffic level; a
  heavier app would want sampling/aggregation.
- **Migration version `000022`** is the next free slot (verified unique);
  watch for parallel branches minting the same number.
# SUMMARY — avatar-placeholder

Accounts with no profile picture used to show a bare `bg-muted` gray disc with
a lowercase single initial (or nothing at all when `display_name` was blank),
so the avatar preview — e.g. in the sidebar "Who to follow" — looked empty.
This branch adds a real placeholder avatar: an uppercase initial on a
username-derived background color, falling back to a person icon when even the
name is missing.

## What was changed and why

- **New shared component `web/src/components/UserAvatar.tsx`** — one place for
  the entire app's avatar rendering, replacing the ad-hoc
  `<Avatar><AvatarImage/><AvatarFallback/></Avatar>` blocks that were
  copy-pasted (with subtly different fallback logic) into 13 files.
  - Props: `src`, `name`, `username`, `alt`, `className` (sizing), and
    `fallbackClassName` (for big avatars like the profile page's `text-6xl`).
  - Fallback shows the first code point of `name || username`, uppercased
    (`Array.from(...)[0]` so emoji/surrogate starts don't break), on a color
    hashed deterministically from `username || name` (12-color palette, all
    legible with white text). Renders a `User` lucide icon if there's no
    initial at all.
  - Only renders `AvatarImage` when a `src` exists (mirrors `getMediaUrl`
    returning `undefined` for empty uuids), so a missing picture always shows
    the placeholder instead of a broken-image attempt.
- **Swapped every avatar usage** (sidebar logoff row, Who-to-follow, feed
  posts, reply dialog, composer, profile header + edit dialog, search, explore,
  notifications, DMs list + conversation + empty state, followers/following,
  lists, list members, hover cards) to `<UserAvatar … />`. Net −57 lines.
- Frontend-only; no server, migration, or API changes.

## Verified

- `npm run lint` (web-tools): 0 errors, 16 warnings — all pre-existing, none in
  the changed files.
- `npm run build` (web-tools, `tsc -b && vite build`): passes. A `React`
  import left in `UserAvatar` (unused under noUnusedLocals) was caught by tsc
  and removed.
- Go tests not re-run — no server-side changes on this branch.

## Reviewer double-checks

- `UserHoverCard` 's fallback resolves through `useFetchProfile` (may be
  undefined → falls back to the `name` prop), same as before.
- The two `ProfilePage` avatars pass explicit `fallbackClassName` sizes
  (`text-6xl` / `text-2xl`); everything else uses the default `font-bold`
  white-on-color letter.
- `ComposeContent` passes only `username` (no display name in UserContext) —
  the initial comes from the username, which is the intended behavior.
# SUMMARY — fix-session-and-static-images

Fixes two live issues reported on the AWS box (`http://100.31.118.41`):
spurious logout with `SESSION_EXPIRED` from `/auth/refresh-token`, and nginx
403s on the fixed-name public assets (`/favicon.ico`, `/gaggle-goose.png`).

## Root causes

**1. Random logout (`SESSION_EXPIRED`)** — two compounding server behaviors:

- The box runs `COOKIE_SECURE=true`, but the site is browsed over **plain
  HTTP**. Browsers refuse to STORE a `Secure` cookie over http, so the browser
  never persists the refresh cookie over `http://<ip>`. Verified live: an HTTP
  register returns `Set-Cookie: refresh_token=…; Secure; SameSite=Lax` and
  curl leaves the cookie jar empty. Sessions could not survive a refresh.
- The refresh-token **rotation theft detector fired on benign stale replays**:
  a replay of an already-rotated token (second tab holding the pre-rotation
  cookie, or a lost refresh response) revokes the **whole session family** →
  `SESSION_EXPIRED` → user logged out. Verified live over HTTPS: refresh T →
  200 + rotate; replay the same T → **exactly**
  `{"data":null,"error":{"code":"SESSION_EXPIRED","message":"session expired"}}`.

**2. 403 on `favicon.ico` / `gaggle-goose.png`** — these files keep FIXED
filenames (unlike content-hashed `/assets/*`), so a stale/corrupt cached `web`
image layer can serve them 403 while the hashed assets refresh fine. Source is
clean (real 0644 PNG/ICO; a fresh local build + running container serve both
200; the live box serves the same hashed bundle yet 403s only the two fixed
names). Not reproducible from current source — box runtime/side artifact;
hardened the deploy so it can't recur and gave a remediation command.

## What was changed

- **`server/internal/handlers/auth_handler.go`** — the refresh-token cookie's
  `Secure` attribute now tracks the actual client scheme via nginx's
  `X-Forwarded-Proto` (http → Secure off, https → Secure on), falling back to
  the configured `COOKIE_SECURE` only when the proxy header is absent (direct
  API access). Plain-HTTP sessions now persist even though prod sets
  `COOKIE_SECURE=true`; HTTPS clients keep Secure cookies.
- **`server/internal/service/auth_service.go`** — replaying an already-rotated
  refresh token from the **same device (user-agent)** is treated as the benign
  concurrent-tab/stale-cookie case: the family's **current** active token is
  rotated forward (`rotateCurrentActiveToken`) and a fresh access token is
  returned — nobody gets logged out. Replay from a different user-agent is
  still theft (family revoked + `SESSION_EXPIRED`), preserving the stolen-token
  detection.
- **`server/internal/store/auth_store.go` + `store.go`** — new
  `GetCurrentActiveToken(ctx, tx, sessionID)` running inside the caller's
  transaction with `FOR UPDATE` so concurrent stale replays serialize (the
  second waits, finds the first's fresh token, and rotates that forward).
- **`server/internal/store`/model note** — `models.RefreshToken.UserAgent`
  (already stored at issuance) is the same-device signal.
- **`deploy/apply.sh`** — `docker compose build --no-cache` (stale dist layers
  can no longer ship wrong fixed-name assets), and the deploy health check now
  asserts `/favicon.ico` + `/gaggle-goose.png` actually return 2xx over HTTPS
  (fails the deploy otherwise).
- **`compose.prod.yaml` / `README.md`** — corrected the `COOKIE_SECURE`
  comment (value is now only the direct-access fallback) and documented the
  behavior + the `--force-recreate web` remediation for a stuck 403 container.
- **`server/internal/testutil/testutil.go`** — `Request` supports `Headers` +
  `UA`; added `NewAppWithCookieSecure`.

## Tests (TDD, red→green)

- `TestRefreshTokenRotation` — same-UA replay of a rotated token now returns
  200 and keeps the session alive (previously 401 + family nuke).
- `TestRefreshTokenReplayFromDifferentDeviceIsTheft` — cross-UA replay still
  401 `SESSION_EXPIRED` and revokes the whole family.
- `TestRefreshConcurrentStaleReplayKeepsSession` — two goroutines refresh the
  same stale token simultaneously; both succeed and the session survives
  (multi-tab regression).
- `TestRefreshCookieSecureFollowsScheme` — cookie Secure follows
  `X-Forwarded-Proto` under every combo of the configured fallback.
- `go vet ./... && go test ./...` all green.

## Reviewer notes

- The same-device signal is the **user-agent string** (IPs are unreliable
  behind the nginx proxy / mobile networks). A stolen cookie replayed with the
  SAME UA slips through as "benign" — accepted trade-off to stop false
  logouts; document if the app ever needs stronger device fingerprints.
- Test harness requests default to UA `""`, which NEVER matches the benign
  branch — tests that exercise it must set a UA explicitly.
- Live-box probes during debugging registered two throwaway users
  (`dbg_1787195238`, `dbg2_1787195271`) — no delete endpoint exists; remove
  manually if wanted.
- 403 fix is deploy-hardening + remediation, not a code change to the assets
  (they're the grok-logo art and already fine in source).

---

# SUMMARY — message-grouping

Glues DM messages sent back-to-back by the same sender into a single bubble
group with one timestamp and flattened inner corners, iMessage/WhatsApp style.

## What was changed and why

- **Grouping window.** `ConversationPage.tsx` computes a per-message group
  position (`start`/`mid`/`end`/`standalone`) in a `groupPositions` memo:
  two messages are "close" when the same sender, same calendar day
  (`getMessageDayKey`), and within 5 minutes of each other
  (`MESSAGE_GROUP_WINDOW_MINUTES`, the same threshold the reference demo used).
  Day boundaries intentionally break a group so a group can never straddle a
  day-divider pill.
- **One timestamp per group.** The per-bubble time line was removed. Instead a
  centered `formatMessageHour` label renders above the first message of each
  group (and standalone messages), matching the reference demo. A pending
  (optimistic, unsent) message shows "Sending…" there instead.
- **Corner rounding** (the curved top/bottom + flat middle look). The chained
  edge of grouped bubbles flattens to a 4px radius; the outer edge keeps the
  full `rounded-2xl` (1rem) bubble radius:
  - `.group-start`: bottom edge flat → `bottom-left` (theirs) / `bottom-right`
    (mine)
  - `.group-mid`: both chained corners flat
  - `.group-end`: top edge flat
  - standalone unchanged. All in `web/src/index.css` next to the existing
    `--chat-gradient` bubble rules. Specificity (0,2,0) beats `rounded-2xl`
    (0,1,0). The `background-attachment: fixed` gradient trick is untouched,
    so a glued stack still reads as one continuous viewport-wide gradient.
- **Spacing.** Removed `space-y-2` from the message list and instead put
  `mb-2` on each group-end/standalone wrapper (and the "Load older" button),
  so bubbles within a group touch and groups keep the previous 8px gap.
- **Sender name.** Incoming bubbles now show `@username` only on the group's
  first message (start/standalone) instead of repeating it on every bubble.

## Files touched

- `web/src/pages/ConversationPage.tsx` — grouping logic + grouped rendering.
- `web/src/index.css` — corner-radius overrides for `.group-*` bubbles.

## Reviewer double-checks

- Verify `npm run build` (tsc) + `npm run lint` — both pass (0 errors).
- Threshold: 5 minutes was lifted from the reference demo; tune
  `MESSAGE_GROUP_WINDOW_MINUTES` if it feels too sticky in real conversations.
- Visual: mid-group messages have fully flat chained corners (4px) — confirm
  the glue looks right with the global gradient at various bubble widths.
- Browser verify against the running stack (no vitest in this repo), e.g. the
  "hey / you around today? / wanted to ask" style burst from the reference.

---
# SUMMARY — research-db-seeding

Audit of how DB seeding / dummy-data creation works today. **Research only — no
code changed.** Full report was delivered to the coordinator in the session
reply; this section is the permanent record.

## How seeding works right now

- **One seed binary**: `server/cmd/seed/main.go`, built into the api image as
  `/app/seed` (`server/Dockerfile:15`) and run **manually** via `make seed`
  (`docker compose run --rm --no-deps --entrypoint /app/seed api`,
  `Makefile:26-27`). Nothing auto-runs it — `docker-entrypoint.sh` only
  migrates then `exec /app/api`; `deploy/apply.sh` (prod) never seeds either,
  which is why a fresh prod EC2 comes up with an empty DB.
- **Idempotency**: guard is `alice@example.com` existing
  (`cmd/seed/main.go:51-54`) — every run after the first exits fast.
- **What the current seed actually creates** (8 users alice…henry, all
  `password123`; honestly small):
  - 8 users via `store.Users.Create` (default profile: display_name=username).
  - Rich profiles (display names/bios/locations/websites/random birthdates)
    via `UpdateUserProfile`.
  - Posts: loop over 16 templates **breaks at `len(users)` = 8**, so only ~8
    top-level posts + 8 replies (`ParentID` set), all `public`, hashtags synced
    by hand (store-level call, no service). No created_at backdating.
  - Follows: 16 **hardcoded integer user-ID pairs** `{1,2}…` + one block
    (1→5). Fragile — see Reviewer double-checks.
  - Media: **4 orphan rows** (random UUID, no file written, attached to
    nothing; `GET /media/{uuid}` will 404 at the fs read).
  - **NOT seeded at all**: likes/reposts/bookmarks/engagement counters, polls
    + votes, DMs/conversations, lists + memberships, badge grants, quotes,
    mention resolution, notifications. (Count columns like followers/likes/
    reposts/bookmarks/replies are maintained by DB triggers — `maintain_likes_count`
    etc. — so raw engagement writes would keep counters consistent.)
- **Admin**: README claims "alice is the seeded admin", but nothing in the
  seed sets it. `000013_add_user_badges.up.sql:37` does
  `UPDATE users SET is_admin=TRUE WHERE username='alice'` **at migration time
  (before alice exists) → 0 rows**. alice is admin only on DBs seeded before
  that migration shipped.
- **The real plan exists but is unimplemented**:
  `docs/superpowers/specs/2026-08-19-seed-strategy-design.md` + summary
  section `# SUMMARY — seed-data-strategy` (spec-only branch). Chosen: a
  `server/internal/seedgen` package (deterministic faker dataset — 30 users,
  ~400 posts spread over 28d, engagement, DMs, lists, badges, generated-PNG
  media) + auto-seed via `SEED_ON_START` in `docker-entrypoint.sh` +
  `compose.yaml`, and a future `cmd/simulate` cron seam. That branch was
  merged; **the code was never written** (no `seedgen` dir, no `SEED_ON_START`
  anywhere, no `gofakeit` dep). The designed `post_store.go` COALESCE
  `created_at` fix is likewise absent (`Create`/`CreateQuotedPost` still omit
  the column).

## Files touched (this branch)

- `.opencode/project-notes.md` — prepended "Seeding audit" notes.
- `SUMMARY.md` — prepended this section. All existing sections kept intact.

## Verification

- Research-only: `go build ./...` and `go vet ./cmd/...` (tools container)
  both clean — seed binary compiles as-is.

## Reviewer double-checks

- **alice-admin gap**: confirm the migration-ordering analysis, and decide
  where the fix lives (seed `IsAdmin=true` is the clean spot — migrations
  can't see users that don't exist at migration time).
- **Hardcoded IDs**: `followPairs`/block `{1,5}` are only correct on a virgin
  DB. Something to fix when the seed gets rewritten.
- Whether the untouched "seed-data-strategy" design is the spec we implement
  next — its backdating needs the `post_store.Create` COALESCE change first.

---

# SUMMARY — settings-appearance-fixes

Settings → Appearance was partly fake UI: two shadcn dropdowns (a light/dark/
system Theme select and a Font Size select) PATCHed the server's
`appearance.*`, but NOTHING in the frontend ever applied them — `ThemeProvider`
only reads localStorage, and font size had no implementation at all. Worse, the
Theme dropdown duplicated the `ThemeToggle` tabs already sitting inside
`ThemeCustomizer`, and the dropdown didn't match the customizer's bespoke
Tabs/button-grid UI.

## What was changed and why

- **Removed the two duplicate dropdowns** from the Appearance card in
  `SettingsPage`. Appearance controls now live solely in `ThemeCustomizer`
  (ThemeToggle tabs + theme catalog + font picker + a new font-size row), so
  there is exactly ONE light/dark/system control and it matches the existing
  UI.
- **Font size now actually works.** `settings.appearance.fontSize` only got
  PATCHed (`--app-font-sans` just sets font *family*). Added a real scale:
  - `ThemeContext`: new `fontSize` state (`small|medium|large`, persisted to
    `localStorage["vite-ui-font-size"]`, default `medium`) that sets
    `--app-font-size` on `<html>`.
  - `index.css`: `:root` declares `--app-font-size: 16px` and
    `font-size: var(--app-font-size)`. Every Tailwind size is rem-based, so
    the whole UI scales proportionally (14 / 16 / 18px).
  - `ThemeCustomizer`: a "Font Size" row of Small/Medium/Large tabs (radix
    Tabs — same visual style as the ThemeToggle row above it).
- **Appearance settings now persist AND apply.** `ThemeToggle` +
  `ThemeCustomizer` PATCH `appearance.theme` / `appearance.fontSize` to the
  account on change (the theme catalog id + font family stay localStorage-only —
  the server settings model has no fields for them). New `AppearanceSync`
  component (mounted under `I18nProvider`) reads the shared `['settings']`
  query and adopts the account's persisted `appearance.*` into `ThemeContext`
  on load — so the setting follows the account across browsers, not just one
  browser's localStorage. A choice the user made this session
  (`appearanceTouched` flag on the context setters) always wins over a
  refetch.
- Removed the now-dead `settings.appearance.{theme,light,dark,system,fontSize,
  small,medium,large}` i18n keys from `en`/`es`/`fr`/`de`.

## Files touched

- `web/src/pages/SettingsPage.tsx` — deleted the duplicate theme + font-size dropdowns
- `web/src/contexts/ThemeContext.tsx` — `fontSize` state + `--app-font-size` + `appearanceTouched` guard; exported `Theme` type + `FONT_SIZES`
- `web/src/index.css` — root font-size scales from `--app-font-size`
- `web/src/components/ThemeCustomizer.tsx` — Font Size tabs; persists theme/fontSize to the server
- `web/src/components/ThemeToggle.tsx` — optional `onThemeChange` callback; controlled `value`
- `web/src/components/AppearanceSync.tsx` (new) — adopts server appearance into ThemeContext
- `web/src/App.tsx` — mounts `AppearanceSync`
- `web/src/i18n/{en,es,fr,de}.ts` — dropped unused appearance sub-keys

## Verification

- `npm run lint` 0 errors (17 pre-existing react-refresh warnings); `npm run
  build` (tsc + vite) passes (pre-existing >500kB chunk warning only).
- Playwright smoke (vite dev on :5199 proxying the live api): Appearance card
  has **0** Select/combobox dropdowns; Font Size renders as a tab control;
  clicking **Large** → computed `html` font-size 16px → 18px, localStorage +
  `PATCH /users/settings` both update; clicking **Dark** → `.dark` class +
  localStorage + PATCH `theme:"dark"`; after a full reload the appearance is
  re-adopted from the server. Also observed the sync applying across browser
  contexts (a brand-new context picked up the account's persisted dark/large).
  The account's settings were reset to system/medium at the end of the run.
- No server or migration changes (the model already had
  `appearance.theme`/`appearance.fontSize`).

## Reviewers should double-check

- Font size is implemented as **root font-size scaling**, so ALL rem-based UI
  (buttons, sidebar, spacing) zooms ±12.5%, not just typography. Intended —
  it's the standard way to scale a Tailwind app; if "font size" should only
  change body text, that's a substantially larger refactor.
- Theme catalog (`themeId`) + font family remain localStorage-only. Only
  `theme` + `fontSize` sync to the account. Extending the server settings
  schema (and defaults/migrations docs) for the other two is possible future
  work.
- `AppearanceSync` adopts once per session (guarded by `appearanceTouched`);
  a theme change made on another device is picked up on the next page load,
  not live mid-session.
# SUMMARY — settings-page-bg-split

On the Settings page, scrolling revealed a hard horizontal "split" in the
background at exactly `100vh` — the area above the fold showed the app's tinted
panel color, and everything below it showed a different, raw background color.
The split occurred on **every theme** (worst on themes with big background-
vs-sidebar contrast, e.g. comic's halftone dots / dark themes) and also affects
any other page whose content scrolls past one viewport (feed at short
viewports, mentions, profile, …).

## Root cause

A layout regression from `improve-message-flow` (commit `33e0f97`). The main
content column in `web/src/layout/SocialMediaLayout.tsx` paints the app's page
background (`bg-background/25` — a 25% tint of `--background` over
`bg-sidebar`, and on the comic theme a solid cream + halftone pattern). That
branch changed the column from `min-h-screen` to `h-screen flex flex-col`
(with a `flex-1 min-h-0` child), so the message pages could get fixed-height,
internally-scrolling threads. The side effect: the column's box — and therefore
its background — is clamped to **exactly one viewport (100vh)**, while non-
message pages (Settings, Feed, …) have no internal scroller, so their content
overflows past the column. Once the document scrolls more than a viewport, the
region below the column exposes the raw `bg-sidebar` (outer container) / `body`
`bg-background`. Verbatim from the old diff: the column went from
`... bg-background/25 min-h-screen px-6 pb-16 md:pb-0` to
`... h-screen flex flex-col`.

## What was changed and why

One line in `web/src/layout/SocialMediaLayout.tsx` line 208:

- `h-screen` → `min-h-screen self-start` (keeping the `flex flex-col` + inner
  `flex-1 min-h-0` wrapper that message pages depend on).

- **`min-h-screen`** lets the column grow with its content, so the
  `bg-background/25` tint (and the comic halftone override) extends the full
  height of every page instead of stopping at 100vh → no split.
- **`self-start`** (align-self: start) is the second half of the fix: without
  it, the grid item stretches to the row height, which on short viewports is
  driven taller by the sidebar rails (~100–200px). For the fixed-height
  message pages that stretch would push the composer/input below the fold.
  `self-start` keeps the column at max(content, 100vh), so message pages keep
  their composer pinned in-view and threaded scroll exactly as before.

Verified against a live browser (playwright, vite dev on :5174 → :2021):

- Before: `/settings` scroll height 1627 while the tinted column covered only
  `[0, 900]` (or `[0, 700]` at a shorter viewport) → seam exactly at 100vh.
  Feed at a 700px viewport: column covered `[0,700]`, content ran to 898.
- After: `/settings` column bottom = 1627 = scrollHeight (full coverage);
  full-page pixel sampling shows the bottom-most background gap renders the
  identical tinted color `srgb(248,245,238)` as the top gaps — one uniform
  background. Feed column = full height whenever content is taller than a
  viewport.
- No message regression: `/messages` and `/messages/<id>` keep a 100vh column,
  bounded internal thread scroller, page-level scroll unchanged (~198px at a
  700px viewport, the pre-existing sidebar-rail overflow), and the composer is
  visible at scrollY=0 (`inputVisible: true`).

## Files touched

- `web/src/layout/SocialMediaLayout.tsx` — main-content column: `h-screen` →
  `min-h-screen self-start`.

## Reviewer double-checks

- The residual sub-viewport band below the *last* piece of content on very
  short pages (message pages, sparse feeds) is the pre-existing "sidebar rails
  taller than the viewport" overflow already documented in project-notes —
  unchanged by this branch (the column is no longer stretched into that band).
- `flex flex-col` + `flex-1 min-h-0` wrapper is intentionally untouched: it is
  what gives ConversationPage/MessagesPage their fixed-height internal scroll.
- No server/DB/migration changes → no migration-version risk.
- Frontend verification = `npm run lint` (0 errors, 16 pre-existing warnings)
  + `npm run build` (passes, pre-existing chunk-size warning only).

---
# SUMMARY — snappy-ux


Makes the web app feel immediate: sent messages appear instantly
(optimistic), navigating between pages no longer round-trips on every mount,
and incoming DMs show up live while you're reading a conversation.

## What was changed and why

- **Optimistic message sending (the big UX win).** `useSendMessage`
  (`web/src/hooks/useDms.ts`) had no `onMutate` — a sent message only appeared
  after the POST round-trip *and* a follow-up `['dm-messages']` refetch, which
  is why sending felt broken/slow. It now cancels in-flight message queries,
  prepends a `pending` placeholder message into the cached pages, and returns a
  context for rollback. `ConversationPage` clears the composer immediately on
  send (restores it on error) and renders the placeholder with a "Sending…"
  timestamp + reduced opacity. `onSuccess` swaps the placeholder for the
  server-confirmed message (matching on its negative temp id) before the usual
  invalidation, so there's no flicker or duplicate. `onError` restores the
  previous cache state and the user's draft.
  - New-`conversationId` adds `sender` (from `UserContext`) to the mutation
    variables so the placeholder can render correctly; `Message` gained a
    `pending?: boolean` field.
  - `/messages/new` conversations still must wait for the server (the
    conversation id only exists after creation) — the composer still clears
    instantly and navigates on success.
- **Snappier navigation.** The QueryClient had no default `staleTime`, so
  React Query treated every cached page as stale and refetched on every mount
  (spinners/skeletons on each visit). Added a global `staleTime: 60_000`
  (`web/src/App.tsx`) so data is served from cache within a minute; refetches
  still happen in the background for fresh data. Also prefetch the two most
  visited destinations in `SocialMediaLayout`: notifications and DM
  conversations are fetched at layout mount, so first navigation to those
  pages is instant instead of a round-trip.
- **Live DMs.** The SSE handler only invalidated `dm-unread-count` +
  `dm-conversations`, so a message arriving mid-conversation never appeared.
  It now also invalidates `['dm-messages']`.
- The notifications infinite query is now centralized in a new hook
  (`web/src/hooks/useNotifications.ts`) shared by `NotificationsPage` and the
  layout prefetch, so prefetch populates the correct `InfiniteData` shape.

## Files touched

- `web/src/hooks/useDms.ts` — optimistic `useSendMessage` + `sender`/`conversationId` variables
- `web/src/pages/ConversationPage.tsx` — instant clear, draft restore, `Sending…` indicator
- `web/src/types/api.ts` — `Message.pending?`
- `web/src/App.tsx` — default `staleTime: 60s`
- `web/src/layout/SocialMediaLayout.tsx` — prefetch notifications + conversations
- `web/src/hooks/useNotifications.ts` — new shared infinite-query options/hook
- `web/src/pages/NotificationsPage.tsx` — uses the shared hook
- `web/src/contexts/NotificationsContext.tsx` — SSE invalidates `['dm-messages']`

## Reviewers should double-check

- The optimistic placeholder swap relies on the temp message still being in
  cache in `onSuccess`; if a background refetch completes first, the replace is
  a no-op and the refetch already reconciled — no duplication either way.
- The `60s` global staleTime is a behavior change for every query (only the
  feed was `Infinity` before). If any page needs fresher data, prefer a
  per-query `staleTime` override over lowering the default.
- Prefetching adds two GETs on authenticated app load (notifications,
  conversations). Cheap, but it's a per-session cost on every full page load.
- No server code changed; frontend verification = `npm run build` + `npm run
  lint` (repo has no frontend test runner).

---
# SUMMARY — nickname-default (follow-up: no fake birthday for unset profiles)

A brand-new account's profile showed **"Born January 1, 0001"** even though no
birthday is set. Root cause: the API serializes an unset `birth_date` (DB
`NULL` in `user_profiles`) as the Go zero `Date` — the string `"0001-01-01"` —
and `ProfilePage` formatted and rendered it as a real birthday.

## What was changed and why

Frontend-only fix in `web/src/pages/ProfilePage.tsx` (per decision: leave the
API wire format alone, handle the "not set" sentinel on the client):

- Added `UNSET_BIRTH_DATE = "0001-01-01"` + `isUnsetBirthDate()` helper: treat
  that sentinel (or empty/null) as "no birthday set".
- **Display** — `formatDate` now returns `""` for the sentinel, so the existing
  `{formatDate(...) && <…>Born …}` guard hides the "Born" row for users who
  haven't set a birthday.
- **Edit dialog** — the Date of Birth input is seeded with `""` instead of
  `"0001-01-01"` when unset, so opening the editor no longer shows the fake
  date (and a save without touching it sends `""` rather than the sentinel).

## Files touched

- `web/src/pages/ProfilePage.tsx`

## Verification

- `docker compose --profile tools run --rm web-tools npm run build` — passes
  (tsc + vite). `npm run lint` — 0 errors.
- Playwright headless (vite dev on :5174 proxied to the live api) — new
  registered user's profile shows no "Born" text; their edit dialog's Date of
  Birth field is empty; regression: alice (real birthday) still shows
  "February 3, 1994" and her editor keeps the real date.

## Reviewer double-checks

- This relies on the API always returning `"0001-01-01"` (confirmed live: a
  fresh account's `GET /users/nigga` returns `birth_date=0001-01-01`). If the
  server ever switches to `null` for unset, the `!dateString` branch already
  covers it.
- Deliberately frontend-only / no server or DB change: the user is wiping dev
  data, so legacy rows were explicitly out of scope (the earlier display-name
  backfill migration `000022` was removed for the same reason).
---

# SUMMARY — nickname-default

New accounts used to be created with an **empty display name** (`display_name`)
— the "nickname" the app shows next to a username. They walked around "naked":
posts, avatars, DMs, notifications, the sidebar account row, etc. rendered a
blank name (and `name={author.display_name}` alt / `charAt(0)` fallbacks had
nothing to show) until the user went into profile edit and typed one.

## What was changed and why

- **Store-level creation default** (`server/internal/store/user_store.go`,
  `Users.Create`): the user_profiles INSERT hardcoded `display_name = ''`
  (line 178). It now inserts `user.Username` — every new profile's nickname is
  its username until the user changes it. `models.User` carries no display
  name and the register request has no nickname field, so the store is the one
  seam all creators go through (register, admin `CreateUser`, seed). The seed
  still overrides with its fancified names afterwards, unchanged. Usernames are
  ≤ 16 chars so the value always fits the 50-char column (which is also why
  that column is `NOT NULL` with no default — the INSERT was bare).
- **Regression test** (`server/internal/handlers/integration_test.go`,
  `TestProfileLifecycle`): strengthened — a freshly registered user's
  `GET /users/me` must return `display_name == username` (was only checked for
  the key's existence before).
- Note: a backfill for already-created rows was drafted (`000022`) but removed
  — existing accounts are being wiped, so only the creation default ships.

## Files touched

- `server/internal/store/user_store.go` — default display_name to username in
  `Users.Create`
- `server/internal/handlers/integration_test.go` — `TestProfileLifecycle`
  assertion

## Verification

- `go build ./...`, `go vet ./...` — clean (tools container).
- `go test ./...` — all packages pass. `TestProfileLifecycle` passes with the
  new username-default assertion.
- Frontend untouched (server data is now always non-empty, which existing
  `display_name || username` fallbacks still handle).

## Reviewer double-checks

- The store write still passes every other profile column explicitly; only the
  `$2` display_name argument changed from `""` to `user.Username`.
- No migration ships on this branch (the values are produced by the store), so
  there is zero migration-version collision risk at /merge-all.
---
# SUMMARY — web-healthcheck

Deploy gave a false-negative "health check FAILED" after the enable-https
merge: `deploy/apply.sh` fired a single-shot `curl -kfsS
https://localhost/swagger/doc.json` immediately after `docker compose up -d`.
Since the web container's entrypoint now generates a self-signed cert before
nginx listens, the probe raced container boot (curl "Send failure: Broken pipe"
0.2s after `web Started`). The app was actually fine — the workflow just kept
the box on the previous SHA.

## What was changed and why

- **Native `healthcheck` on the `web` service** (`compose.yaml`): rides the same
  proxied path the deploy probes — `wget -qO-
  http://127.0.0.1/swagger/doc.json` — so "web healthy" == nginx serving the
  SPA *and* the proxy to the api, not merely that a socket is bound. Params
  mirror the existing `api` healthcheck (5s/5s/20/10s).
- **`deploy/apply.sh`: `up -d` → `up -d --wait --wait-timeout 300`**: compose
  now blocks until every service reports healthy (or fails after 5 min),
  eliminating the race by construction instead of a retry loop. Services
  without a healthcheck (certbot) count as ready once running. The existing
  `curl` stays as the final gate over the real HTTPS production path.
- No retry/poll hacks — health is expressed natively and compose does the
  waiting.

## Files touched

- `compose.yaml` — added `web.healthcheck`.
- `deploy/apply.sh` — `up --wait` in `deploy()` + comment on `health_check()`.

## Verification

- `docker compose -f compose.yaml config -q` and `-f compose.yaml -f
  compose.prod.yaml config -q` both parse.
- Healthcheck command proven against the live web container (same image built
  here): `127.0.0.1` PASS. `localhost` FAILS (resolves to `::1`, nginx binds
  IPv4 only) — hence `127.0.0.1` in the test and the comment explaining why.
- Full isolated stack (`up --wait` with the healthchecked web + migrated api +
  db + redis) reached all-Healthy in ~17s, exit 0 — i.e. the exact condition
  the deploy waits for.
- `bash -n deploy/apply.sh` clean.

## Reviewer double-checks

- **IPv6/localhost trap**: keep `127.0.0.1` in the healthcheck. `localhost`
  resolves to `::1` inside the container and busybox wget can't reach an IPv4
  nginx bind → a false-unhealthy check would make `up --wait` hang for
  `--wait-timeout` then fail. The comment in compose.yaml explains this.
- **`--wait-timeout 300`** covers cold DNS/pull + 20×5s api healthcheck retries
  with margin; the box already implicitly needs a current compose (the
  enable-https overlay uses `ports: !reset []`, compose ≥ 2.23), and `--wait`
  needs compose ≥ 2.20 — satisfied.
- The healthcheck intentionally depends on the api (swagger is proxied). That's
  the point: web depends_on api:service_healthy anyway, so web's health only
  starts being evaluated after api is already healthy.
- Deploy fragment is exercised end-to-end by an isolated scratch stack that was
  torn down (`down -v`); no test artifact left in the tree.

---
# SUMMARY — grok-logo

Replaces the page's logo art (favicon + sidebar icon) with the new
`grok_cutout` master. The user modified the original logo image; the new art is
a background-transparent cutout stored in
`/home/bau/Programming/svg-img/new-stuff-here/grok_cutout.svg` (vector master,
1024²) / `grok_cutout.png` (4096² raster of the same art).

## What was changed and why

- Generated two web assets from `grok_cutout.svg` and swapped them in place,
  keeping the existing filenames so **no code changes were needed**:
  - `web/public/gaggle-goose.png` (80×80) — the brand mark used by the App Logo
    block in `SocialMediaLayout.tsx:126` (rendered `w-10 h-10 rounded-full`
    = 40px circle, so 2× = retina).
  - `web/public/favicon.ico` (16/32/48/64 ICO frames) — referenced by
    `web/index.html:5`.
- **The cutout art is not centered**: its opaque content is flushed to the
  bottom-right corner (SVG bbox `793x931+231+93`; the canvas corner pixel is
  opaque). Dropping it in raw would clip part of the logo under the sidebar's
  circular crop. So the assets were re-anchored: trim → re-center into a square
  canvas → scale so the art's max reach from center is ≈1.045× the inscribed
  circle radius, matching the fill ratio of the previous goose asset (measured
  41.8px reach vs 40px radius). Verified the new 80px asset's reach is 41.9px.
- Master choice: used the **SVG** (vector, crisp at any size via `rsvg-convert`),
  consistent with how the old goose assets were generated. The PNG is just a
  4096² raster of the same paths and produces the same result.

## Files touched

- `web/public/gaggle-goose.png` (replaced binary)
- `web/public/favicon.ico` (replaced binary)
- `.opencode/project-notes.md` (new note: master location + regeneration recipe)

## Verification

- `npm run build` (vite): passes; both new assets emitted into `dist/` and byte-
  identical to `public/`.
- `npm run lint`: 0 errors (16 pre-existing react-refresh warnings, none in
  changed files — no TS/JS touched).
- Analyzed the generated PNGs (`magick identify` + alpha threshold): content
  bbox centered, transparent corners on the favicon frames, 80px logo reach
  1.048× radius (target 1.045).

## Things a reviewer should double-check

- **Visual QA**: I can't render images in this environment, so eyeball the
  logo at `http://localhost:5173` after a rebuild — specifically the circular
  40px sidebar icon on light + dark themes and the browser-tab favicon. If it
  reads too small/large, tune the `694x815` resize (raise/lower = bigger/smaller
  in the circle) and retrim/regenerate.
- The 80px asset name is still `gaggle-goose.png` (unchanged reference) even
  though the art is no longer a goose — renaming was deliberately skipped to
  avoid touching TS/HTML. A future branch could rename it to e.g. `gaggle-logo.png`.
- Masters stay in `/home/bau/Programming/svg-img/new-stuff-here/` (outside the
  repo), matching the existing `goose_max.svg` convention; only the raster web
  assets are committed.
- `package-lock.json` untouched this time (browser/`npm run build` only — no
  `npm install` in the web-tools container).

---
# SUMMARY — f5-loading-ui

Kills the ugly bare "Loading..." text on full page reloads (F5). Every
authenticated page (feed, profile, messages, settings, …) showed a lone
raw-text `Loading...` in the corner of a white page while the auth bootstrap
(refresh-token round trip) was in flight; on a slow/stalled AWS box that state
could persist indefinitely because the requests had no client-side timeout.

## What was changed and why

- **New branded boot splash** (`web/src/components/AppSplash.tsx`): full-screen,
  centered goose logo + "Gaggle" wordmark + spinner, themed via the app's CSS
  vars (`bg-background`, `text-primary`) so it matches light/dark. Replaces the
  bare text in `SocialMediaLayout.tsx` (`token === undefined` branch) — the only
  auth gate, so it covers every layout page.
- **Pre-JS splash in `web/index.html`**: branded markup inside `#root` (inline
  styles + `gaggle-spin` keyframes) covers the F5 gap *before* the bundle
  loads/parses — the old behaviour was a blank white page there. React
  overwrites the `#root` children on mount, so the swap is seamless.
- **Bootstrap timeout** in `web/src/contexts/AuthContext.tsx`: the bootstrap
  refresh-token POST and `/users/me` GET now carry an `AbortSignal.timeout`
  (10s, with fallback where unsupported). If the box stops answering, the
  promise rejects and the app falls back to the logged-out state (login page)
  instead of hanging on the boot screen forever. The single-flight
  `refreshPromiseRef` cleanup (`.finally`) is unchanged.

## Files touched

- `web/src/components/AppSplash.tsx` (new)
- `web/src/layout/SocialMediaLayout.tsx` (use AppSplash for auth loading)
- `web/index.html` (pre-JS branded splash + keyframes)
- `web/src/contexts/AuthContext.tsx` (bootstrap timeout + signal plumbing)

## Review notes

- Tradeoff: a session whose refresh takes >10s is treated as signed-out (login
  redirect) rather than waiting forever. 10s is generous for the AWS box
  (nginx + API are same-host); revisit if users on very slow links complain.
- `AbortSignal.timeout` is skipped on browsers without it (older Safari) —
  those get the old no-timeout behaviour, which is the conservative choice.
- Verified: `npm run build` (tsc + vite), `npm run lint` (0 errors; the two
  AuthContext warnings are pre-existing on the base branch), and a playwright
  smoke test against `vite preview` — pre-JS splash served in index.html,
  `/login` renders app UI, `/` resolves (redirects to login) instead of
  parking on the splash.
---

Deploy gave a false-negative "health check FAILED" after the enable-https
merge: `deploy/apply.sh` fired a single-shot `curl -kfsS
https://localhost/swagger/doc.json` immediately after `docker compose up -d`.
Since the web container's entrypoint now generates a self-signed cert before
nginx listens, the probe raced container boot (curl "Send failure: Broken pipe"
0.2s after `web Started`). The app was actually fine — the workflow just kept
the box on the previous SHA.

## What was changed and why

- **Native `healthcheck` on the `web` service** (`compose.yaml`): rides the same
  proxied path the deploy probes — `wget -qO-
  http://127.0.0.1/swagger/doc.json` — so "web healthy" == nginx serving the
  SPA *and* the proxy to the api, not merely that a socket is bound. Params
  mirror the existing `api` healthcheck (5s/5s/20/10s).
- **`deploy/apply.sh`: `up -d` → `up -d --wait --wait-timeout 300`**: compose
  now blocks until every service reports healthy (or fails after 5 min),
  eliminating the race by construction instead of a retry loop. Services
  without a healthcheck (certbot) count as ready once running. The existing
  `curl` stays as the final gate over the real HTTPS production path.
- No retry/poll hacks — health is expressed natively and compose does the
  waiting.

## Files touched

- `compose.yaml` — added `web.healthcheck`.
- `deploy/apply.sh` — `up --wait` in `deploy()` + comment on `health_check()`.

## Verification

- `docker compose -f compose.yaml config -q` and `-f compose.yaml -f
  compose.prod.yaml config -q` both parse.
- Healthcheck command proven against the live web container (same image built
  here): `127.0.0.1` PASS. `localhost` FAILS (resolves to `::1`, nginx binds
  IPv4 only) — hence `127.0.0.1` in the test and the comment explaining why.
- Full isolated stack (`up --wait` with the healthchecked web + migrated api +
  db + redis) reached all-Healthy in ~17s, exit 0 — i.e. the exact condition
  the deploy waits for.
- `bash -n deploy/apply.sh` clean.

## Reviewer double-checks

- **IPv6/localhost trap**: keep `127.0.0.1` in the healthcheck. `localhost`
  resolves to `::1` inside the container and busybox wget can't reach an IPv4
  nginx bind → a false-unhealthy check would make `up --wait` hang for
  `--wait-timeout` then fail. The comment in compose.yaml explains this.
- **`--wait-timeout 300`** covers cold DNS/pull + 20×5s api healthcheck retries
  with margin; the box already implicitly needs a current compose (the
  enable-https overlay uses `ports: !reset []`, compose ≥ 2.23), and `--wait`
  needs compose ≥ 2.20 — satisfied.
- The healthcheck intentionally depends on the api (swagger is proxied). That's
  the point: web depends_on api:service_healthy anyway, so web's health only
  starts being evaluated after api is already healthy.
- Deploy fragment is exercised end-to-end by an isolated scratch stack that was
  torn down (`down -v`); no test artifact left in the tree.

---
# SUMMARY — news-preview

Adds the ability to attach a single news article link to a post. The post then
renders a card with the article's headline + photo preview (OpenGraph-style),
and the composer has an "attach news" flow that unfurls a pasted URL before
posting. There was no news concept in the codebase before this — it follows the
existing poll-attachment pattern.

## What was changed and why

- **Posts can carry one news link.** New `post_news` table (one row per post, so
  a post may attach at most one article; `post_id PRIMARY KEY`, cascading delete).
  Columns: `url`, `title`, `image_url`, `site_name`. A post with no news simply
  has no row, and the API omits the field.
- **`GET /links/preview` unfurls a URL in the composer.** Because a client-side
  scrape would hit CORS, the server fetches the link and extracts OpenGraph
  metadata (`og:title`, `og:image`, `og:site_name`, falling back to `<title>`),
  resolving relative image URLs to absolute. New package `internal/linkmeta`
  (`golang.org/x/net/html` seen in the wild: both `property=` and `name=` —
  attempts are blocked by redirects/cap).
- **Snapshots travel with the post.** The client fetches `/links/preview` at
  compose time and sends the resulting title/image/site_name in the create
  payload; the server persists that snapshot rather than re-scraping server-side.
  Models: `NewsLink` for the wire/feed shape, `NewsPayload` on `models.Post`
  (`json:"-"` so it never leaks in APIs), `CreatePostNewsPayload` for create
  validation (`required,url,max=2000` on url, `max=300` title, `omitempty,url`
  image, `max=200` site_name).
- **All feeds hydrate news.** `hydrateNews` batches one `GetForPosts` per page
  (using `pq.Array`) alongside the existing `hydratePolls` calls — home feed,
  user feed, replies/media/likes/bookmarks/pins/quotes, post detail + ancestors/
  descendants/chain, and search results.
- **Allowed on quote posts, disallowed on replies.** Quote and top-level posts
  accept news; replies do not (mirrors the poll rule — enforced both in the
  composer, which hides the attach button when replying, and server-side in
  `PostService.Create`, which rejects `news` + `parent_id` with a 400).
- **Frontend.** New `NewsCard` component (image header + site name + title;
  hides the image on load error; link opens in a new tab). Rendered under the
  post body in `FeedPost` and as a removable preview in `ComposeContent`. The
  composer toolbar gains a link button that opens a URL input; on "Preview" it
  calls `/links/preview` and swaps in the card, which is sent with the post.

## Files touched

- `server/cmd/migrate/migrations/000021_add-post-news.{up,down}.sql` — `post_news`
  table (new; `000021` is above the highest existing `000020` so no version
  collision).
- `server/internal/models/post.go` — `NewsLink`, `CreatePostNewsPayload`,
  `NewsLinkPreviewRequest`, `News` on `FullPost`, `NewsPayload` on `Post`,
  `News` on `CreatePostPayload`.
- `server/internal/store/news_store.go` (new) + `store.go` — `News.Create`,
  `News.GetForPosts`, wired into the `Store.News` interface.
- `server/internal/linkmeta/linkmeta.go` + `linkmeta_test.go` (new) — OpenGraph
  unfurl, URL-only fallback on fetch failure. `server/go.mod`: `golang.org/x/net`
  promoted from indirect to direct.
- `server/internal/service/post_service.go` — `hydrateNews` + call sites,
  `NewsPayload` persistence in `Create`, `PreviewLink` (delegates to linkmeta).
- `server/internal/service/{service,search_service,list_service}.go` — `PreviewLink`
  on the `Posts` interface, news hydration in search + list feeds.
- `server/internal/handlers/post_handler.go` — `CreatePost` maps news,
  new `PreviewLink` handler (swagger-annotated).
- `server/internal/handlers/post_engagement_handler.go` — `Quote` maps news.
- `server/internal/api/router.go` — `POST /links/preview` (protected).
- `server/internal/handlers/integration_test.go` — `TestNewsAttachmentLifecycle`
  (create round-trip, detail + home feed + user feed + search hydration, absence
  on plain posts, reply rejection).
- `web/src/types/api.ts` — `NewsLink`, `Post.news`, `CreatePostPayload.news`.
- `web/src/api/posts.ts` — `previewLink`.
- `web/src/components/PollCard.tsx` — added `NewsCard` export (colocated with the
  other post-attachment card).
- `web/src/components/FeedPost.tsx` — renders `NewsCard` under the post body.
- `web/src/components/ComposeContent.tsx` — attach-news toolbar button, URL
  input, preview via `/links/preview`, removable card, sent with the post.
- `web/package-lock.json` — unchanged deps (lockfile touched by `npm install`).
- `server/docs/{docs.go,swagger.json,swagger.yaml}` — regenerated via swag
  (includes `POST /links/preview`).

## Verification

- Backend: `go build ./...`, `go vet ./...`, `go test ./...` all pass
  (handlers 35s, linkmeta ok).
- Frontend: `npm run build` (tsc + vite) and `npm run lint` pass (0 errors;
  only pre-existing warnings).

## Things a reviewer should double-check

- News is disallowed on replies (composer hides the attach button when quoting/
  replying). Confirm that's the desired scope.
- The snapshot approach means a post keeps whatever the composer previewed at
  create time, even if the article's page changes later. If metadata should
  refresh, a future job could re-unfurl existing `post_news` rows.
- `internal/linkmeta` reads at most `2 MiB` of the fetched body and times out
  after 8s; a slow/unreachable link degrades to a URL-only card rather than
  failing the post. Image loading in `NewsCard` is client-side and may be
  blocked by hotlink protection on some sites; the card still shows the
  headline + site name.
- `web/package-lock.json` shows modified; deps are unchanged (only the install
  timestamp/registry metadata). Verify no stray dep changes before merging.
---
# SUMMARY — settings-language

Makes the Settings language selector actually switch the UI language and
persist, and seeds a new account's language from the browser before an account
exists. No i18n library added — a lightweight dependency-free layer mirrors the
existing `ThemeContext` pattern.

## What was changed and why

### Server: register seeds the `user_settings` row
- `RegisterRequest` (`server/internal/models/auth.go`) gained an optional
  `language` field (`validate:"omitempty,oneof=en es fr de"`). This is the only
  place the browser language can be captured for a brand-new account: Settings
  is only reachable once logged in.
- `AuthService.Register` now accepts `language` and seeds the user's settings
  row atomically with the user, via a new `Users.CreateSettings` store method
  (`server/internal/store/store.go` / `user_store.go`). The seeded JSONB is the
  full defaults object (mirroring the `user_settings` column default) with the
  requested language substituted, so notification/privacy/appearance defaults
  are never lost. `CreateSettings` uses `ON CONFLICT ... settings ||
  EXCLUDED.settings` so any pre-existing row keeps its keys.
  - `server/internal/service/auth_service.go` — `defaultSettings(language)`
    helper builds the defaults; posted inside the register transaction.
  - `server/internal/handlers/auth_handler.go`, `server/internal/service/
    service.go` — signature threaded through.
- New integration test `TestRegisterSeedsLanguageSetting` in
  `server/internal/handlers/integration_test.go` posts `language: "es"` and
  asserts the created account's `GET /users/settings` returns `es` and the
  notifications defaults are intact.

### Frontend: dependency-free i18n layer + browser-language default
- `web/src/i18n/` — new module. `en.ts` is the source of truth for keys; a
  `WideStrings<T>` mapped type lets `es`/`fr`/`de` (`es.ts`, `fr.ts`, `de.ts`)
  be type-checked against the same shape (a missing/renamed key is a compile
  error). `index.ts` exposes `translate`, `detectBrowserLanguage`
  (`navigator.languages` tags like `es-ES` → `es`, fallback `en`),
  `SUPPORTED_LANGUAGES`, and module-level `setCurrentLanguage`/`getCurrentLanguage`
  so non-React code (session-expiry toast) can translate too.
- `web/src/contexts/I18nContext.tsx` — `I18nProvider` + `useI18n()` returning
  `{ language, setLanguage, t }`.
  - Language resolution: starts at `detectBrowserLanguage()`; once an account
    is logged in it adopts `settings.language` (via the `settings` query,
    `enabled` only when `token` is a string). A manual Settings pick is never
    overwritten mid-session (`isUserSelect` guard), so the whole UI re-renders
    in the new language immediately — no waiting on the PATCH round-trip to
    switch strings.
  - Sets `document.documentElement.lang` (was hardcoded `lang="en"` in
    `index.html`) and pokes `setCurrentLanguage` for non-hook callers.
  - `t(key, params?)` resolves dot-paths and interpolates `{name}` params.
- `web/src/App.tsx` — `I18nProvider` wraps `<Router>` inside `AuthProvider`/
  `NotificationsProvider` (it needs the auth token gate and the settings query).
- `web/src/types/api.ts` — `RegisterPayload` gained `language?: string`.

### Strings translated via `t()`
- Auth pages: `LoginPage` (`Sign in`, reset-password form, validation messages
  are now t()-based via `useMemo` schemas), `SignupPage` (and it now sends
  `language: detectBrowserLanguage()` on register).
- Layout/nav: `SocialMediaLayout` (nav items, trending/who-to-follow cards,
  account dropdown, compose dialog), `MobileNavigation`.
- Settings: `SettingsPage` fully translated; language `<Select>` driven by
  `useI18n().language` so the UI flips instantly while still PATCHing
  `settings.language`.
- Shared components: `ComposeContent` (visibility/poll/alt-text dialogs,
  `Option {n}` interpolation), `FeedPost` (kebab menu, reply/quote/edit/delete
  dialogs, visibility tooltips, follow/block toasts), `UserHoverCard`.
- Toasts: `useSettings` (`settings.updated`/`settings.updateFailed`),
  `AuthContext` (`auth.sessionExpired` via module-level language).

## Files touched

- Server: `models/auth.go`, `handlers/auth_handler.go`,
  `handlers/integration_test.go`, `service/auth_service.go`,
  `service/service.go`, `store/store.go`, `store/user_store.go`
- Web: `src/i18n/{en,es,fr,de,index}.ts` (new), `src/contexts/I18nContext.tsx`
  (new), `src/App.tsx`, `src/types/api.ts`, `src/pages/{Login,Signup,
  Settings}Page.tsx`, `src/components/{ComposeContent,FeedPost,
  MobileNavigation,UserHoverCard}.tsx`, `src/layout/SocialMediaLayout.tsx`,
  `src/hooks/useSettings.ts`, `src/contexts/AuthContext.tsx`,
  `package-lock.json` (docker `npm install` inside web-tools)

## Verification

- `docker compose --profile tools run --rm web-tools npm run build` — passes
  (tsc + vite).
- `docker compose --profile tools run --rm web-tools npm run lint` — 0 errors
  (16 pre-existing fast-refresh warnings, same class as `ThemeContext`).
- `docker compose --profile tools run --rm tools go test ./...` — all packages
  pass, including the new `TestRegisterSeedsLanguageSetting` and existing
  `TestSettings`.
- Migration version uniqueness check on the branch — clean (no new migrations).
---
# SUMMARY — message-conversation-fixes

Three direct-messaging issues reported against the live app:
1. Sending lots of messages made the page grow instead of staying fixed-height
   with an internal scrollbar.
2. `/messages/new?user=henry` (or picking a user from the messages search)
   rendered **"Conversation not found."** — you could not start a conversation
   with anyone.
3. Searching users to message fired a backend request on every keystroke (no
   debounce).

## What was changed and why

- **#2 New-conversation route was broken (real code bug, fixed)**
  (`web/src/pages/ConversationPage.tsx`): the page decided "is this a brand-new
  conversation?" with `conversationIdStr === 'new'`. But `/messages/new` is
  registered as a **static** route (`App.tsx:66`) with no `:conversationId`
  param, so `useParams()` returns `{}` there and `conversationIdStr` is
  `undefined` — never `'new'`. `isNew` was therefore always false on
  `/messages/new`: the component computed `conversationId = Number(undefined)`
  = `NaN`, its `dm-conversation` query was disabled (no data, no error), and
  the fallback rendered "Conversation not found." Fix:
  `const isNew = !conversationIdStr || conversationIdStr === 'new'`
  (line 21). With it, `/messages/new` renders the empty-chat UI against the
  target user and first-send creates the conversation.
- **#3 Message-user search had no debounce (real gap, fixed)**
  (`web/src/pages/MessagesPage.tsx`): `NewMessageComposer` passed `query`
  straight into `useSearchUsers`, so every keystroke hit
  `GET /search?type=users`. The app already ships a `useDebounce` hook
  (`web/src/hooks/useDebounce.ts`; AdminPage uses it at 300 ms, FeedPost at
  150 ms). Wired it in with 300 ms (matching AdminPage) and gated the results
  dropdown on the debounced value so the list doesn't flicker while typing.
  Verified: typing `henr` in a burst fired **4** requests before, **1** after.
- **#1 Page growth (already satisfied by current code — verified, no code
  change)**: the message thread is already a fixed-height, internal-scroll
  container (`flex-1 min-h-0 overflow-y-auto` under an `h-screen flex flex-col`
  main column, added by improve-message-flow). Playwright against the live app:
  on `/messages/1` the thread scroller is bounded (client 670 px vs 1986 px
  content) and the **document height did not change** while sending 12 more
  messages (898 px constant across 38 → 50 messages) at both 800 px and 500 px
  viewport heights. The residual ~98 px of page-level scroll on message pages
  is the shared sidebar column exceeding the viewport (present on every page,
  message-count-independent) — deliberately left untouched as out of scope.

### Follow-up: debounce sweep across the app (same branch)

After fixing the messages search, audited **every** input-driven query in
`web/src` and found two more keystroke-fired searches plus a cleanup:

- **ListPage "Add a user" search** (`web/src/pages/ListPage.tsx`,
  `MemberSearch`): `useSearchUsers(query)` fired `GET /search?type=users` on
  every keystroke (3 requests for a 3-char burst) — identical pattern to the
  MessagesPage bug. Now debounced; 1 request per burst.
- **ExplorePage live post search** (`web/src/pages/ExplorePage.tsx`): the
  search box feed `useSearchPosts(query)` live, so each keystroke hit
  `GET /search?type=posts` (6 requests for a 6-char burst) while ALSO rendering
  an inline results preview. Now debounced (live preview kept per product call);
  the submit → `/search?q=` navigation is unchanged and still uses the raw query.
- **Debounce delay centralized** (`web/src/hooks/useDebounce.ts`): added
  `export const SEARCH_DEBOUNCE_MS = 300` as the single source of truth for
  search debounce (per request "don't hardcode, we may tune later"). Migrated
  the existing hardcoded `300` literals in MessagesPage, AdminPage, and
  BookmarksPage to it. `FeedPost` intentionally stays at 150 ms (its search is a
  client-side, in-memory category filter — no API).

## Files touched

- `web/src/pages/ConversationPage.tsx` — `isNew` detection
- `web/src/hooks/useDebounce.ts` — added `SEARCH_DEBOUNCE_MS = 300`
- `web/src/pages/MessagesPage.tsx` — debounced user search
- `web/src/pages/ListPage.tsx` — debounced member search
- `web/src/pages/ExplorePage.tsx` — debounced live post search
- `web/src/pages/AdminPage.tsx`, `web/src/pages/BookmarksPage.tsx` — use the
  shared `SEARCH_DEBOUNCE_MS` (no behavior change)

## Verification

- Playwright (headless chrome via project-notes "Browser verify recipe") against
  a local `vite dev` serving the worktree code at :5174 (proxied to the live
  api — no shared-container rebuild):
  - `/messages/new?user=henry` and `/messages/new?user=grace`: empty-chat UI
    renders ("You haven't talked to @X yet"), composer enabled; search → pick →
    `/messages/new?user=X` works; sending the first message creates the
    conversation server-side, navigates to `/messages/90`, the message appears
    in the thread, and the conversation shows up in the messages list.
  - Search burst `henr` → exactly 1 request (was 4).
  - Debounce sweep bursts: ListPage member search `hen` → 1 request (was 3);
    ExplorePage live search `sunset` → 1 request (was 6); MessagesPage `henr` →
    1 request (regression check). Results still render after the debounce
    (`@henry` row visible in the ListPage dropdown) and ExplorePage's submit
    still navigates to `/search?q=sunset`.
  - Regression: existing conversation thread still bounded with internal
    scroll; page height unchanged.
  - Before the fix, `/messages/new?user=henry` on the deployed app showed
    "Conversation not found." (bug live-reproduced).
- `npm run lint` (web-tools container): 0 errors — the same 14 pre-existing
  react-refresh warnings as the base branch, none in the changed files.
- `npm run build` (tsc -b + vite): passes; the >500 kB chunk warning is
  pre-existing.

## Reviewer double-checks

- The `/messages/new` fix makes "new" detection independent of the
  `:conversationId` route fallback; confirm all entry points that link to
  `/messages/new?user=...` (profile "Message" button, messages search results)
  land on the empty-chat composer.
- Debounce delay is centralized at `SEARCH_DEBOUNCE_MS = 300` in
  `useDebounce.ts` — tune it in one place if search feels sluggish or too
  eager. `FeedPost`'s 150 ms is deliberate (client-side filter, no API).
- The ExplorePage live-results preview is kept (debounced, per product call);
  if it's ever unwanted, only the `useSearchPosts` line + the posts block need
  removing — the submit-to-`/search` navigation is independent.
- No backend, migration, or test-infra changes (the repo's `web` has no test
  runner — verification is browser-based, consistent with prior branches).
- The ~98 px app-wide sidebar overflow was intentionally not touched (affects
  all pages, not messages; separate concern).
---
# SUMMARY — login-experiments (follow-up: promote step flow to /login)

The chosen keeper design (simple step flow) is now the real login page.
`/login` renders the `StepFlow` variant with a footer slot carrying the
affordances the old page had: **Test sign in** button (dev verification
path), **Forgot your password?** (switches to the existing reset card), and
on sign-up link. The old "Try other login designs" link is gone from `/login`
— the lab remains reachable at `/login-lab` and its variants are untouched
(StepFlow renders no footer there; the new `footer?` prop defaults to
undefined).

## Files touched

- `web/src/pages/login-lab/variants/StepFlow.tsx` — added optional
  `footer?: ReactNode` prop rendered inside the form column.
- `web/src/pages/LoginPage.tsx` — sign-in mode is now `<StepFlow
  footer=...>`; forgot-password mode kept as the reset card.

## Verification

- `npm run lint`: 0 errors; `npm run build`: passes.
- Rebuilt `gaggle-web`; headless checks on live `:5173/login`: step flow
  renders with all footer controls; "Welcome back" color unchanged on an
  error (error itself is red); forgot → reset card → back to login works;
  test sign-in navigates to `/`; `/login-lab` still shows 6 variants with
  no footer on the lab copy.

## Reviewer double-checks

- The promoted page hardcodes `h-screen overflow-y-auto` (mirrors the lab
  pane); on very short viewports content scrolls.
- Test sign-in lives only on the production page footer, not in the lab
  StepFlow variant.
- Login now reuses the exact StepFlow component from the lab — if the lab
  variant evolves, `/login` changes too.

---

# SUMMARY — login-experiments (follow-up: error color + title bugfix)

Two root causes fixed for the step-flow keeper design, found during
headless-browser color measurement on the claude / catppuccin / perplexity
themes:

1. **"Welcome back" changed color with errors.** It was rendered as the
   password field's `FormLabel`, which carries `data-error` and
   `data-[error=true]:text-destructive`, so a field error turned the
   heading text red too. Fixed: it's now a plain `<h2>` (`StepFlow.tsx`).
2. **Errors looked like plain text on the default theme.** `studio-claude`
   light set `--destructive: oklch(0.19 0 106.59)` — a neutral dark gray
   identical to `--card-foreground`, so every `text-destructive` error and
   destructive UI (delete/block items, destructive buttons, alerts) rendered
   in foreground gray. Fixed to a true red `oklch(0.577 0.245 27.325)`
   (`theme-themes.css:27`). All other themes already used real red/pink.

## Files touched

- `web/src/pages/login-lab/variants/StepFlow.tsx`
- `web/src/theme-themes.css`

## Verification

- `npm run lint`: 0 errors; `npm run build`: passes.
- `docker compose build web && up -d web`; headless re-check via the real
  localStorage theme path: on studio-claude / catppuccin-mocha-mauve /
  studio-perplexity (dark) the "Welcome back" computed color is identical
  before and after a password error, and the error message renders a
  saturated red on every theme.

## Reviewer double-checks

- Changing a theme token app-wide affects every `text-destructive` surface
  (not just forms) on the claude light theme — that's the intended fix, but
  worth eyeballing delete/block buttons in the light claude theme.
- The identifier step's prompt ("What's your username or email?") still IS
  a `FormLabel`, so it turns red with errors — that's intended field-error
  behavior, unlike the heading.

---

# SUMMARY — login-experiments (follow-up: SplitStepFlow)

Adds a sixth variant to the login lab: **SplitStepFlow** — the SplitPanel
visual frame (gradient brand panel with tagline + feature list on the left,
form column on the right) combined with the StepFlow interaction (identifier
first, then password; progress dots; back button; per-field validation).
Same `useLoginFlow` hook; registered in `variants/index.ts` under Style,
right after split-panel (`?v=split-step-flow`).

Verified: lint 0 errors, build passes, and a headless render of the live
`:5173` container shows the brand panel, identifier→password advance, back
button, and short-password inline error (no navigation). Rebuilt `gaggle-web`
so it's visible at `http://localhost:5173/login-lab`.

## Files touched (this follow-up)

- `web/src/pages/login-lab/variants/SplitStepFlow.tsx` (new)
- `web/src/pages/login-lab/variants/index.ts` (register variant)

---

# SUMMARY — login-experiments

Frontend-only "login lab" for experimenting with login page designs: a new
`/login-lab` route renders a sidebar of login design variants with a live
full-height preview so the user can compare looks/flows and pick a keeper.
No backend changes.

## What was changed and why

- **`/login-lab` route** (`web/src/App.tsx`): added outside the logged-in
  layout, alongside `/login` and `/signup`.
- **`web/src/pages/login-lab/LoginLabPage.tsx`** (new): sidebar of variants
  (grouped Style vs Flow) + full-height live preview. Selection is persisted
  via URL search param `?v=<id>` (defaults to the first variant) so a
  favourite can be linked/bookmarked; sidebar buttons switch instantly.
- **5 variants** under `web/src/pages/login-lab/variants/` (one
  self-contained component each, registered in `variants/index.ts`):
  - `SplitPanel` — brand/marketing panel on one side, form on the other.
  - `Glassmorphism` — frosted card + animated gradient blobs (new
    `login-lab-drift` keyframes in `web/src/index.css`).
  - `CenteredBrand` — big wordmark + compact centered form.
  - `Minimal` — quiet, underline inputs, lots of whitespace.
  - `StepFlow` — flow variant: identifier first, then password, with
    progress dots and back navigation.
- **`web/src/hooks/useLoginFlow.ts`** (new): extracted the login submit flow
  (zod schema, `useLoginMutation`, `/users/me` fetch, `setUser`, toast,
  `navigate('/')`) out of `LoginPage.tsx:43-79` so every variant runs the
  exact same authentication — the variants only change the chrome.
- **`web/src/pages/LoginPage.tsx`**: refactored to use `useLoginFlow` (keeps
  the forgot-password toggle and the "Test sign in" button); added a small
  "Try other login designs" link to `/login-lab` under the form.

## Why purely frontend

The variants reuse the existing `POST /auth/login` + `GET /users/me` flow
via the same hook — no API surface changed, so no server work or migration.

## Files touched

- `web/src/hooks/useLoginFlow.ts` (new)
- `web/src/pages/login-lab/LoginLabPage.tsx` (new)
- `web/src/pages/login-lab/variants/index.ts` (new)
- `web/src/pages/login-lab/variants/SplitPanel.tsx` (new)
- `web/src/pages/login-lab/variants/Glassmorphism.tsx` (new)
- `web/src/pages/login-lab/variants/CenteredBrand.tsx` (new)
- `web/src/pages/login-lab/variants/Minimal.tsx` (new)
- `web/src/pages/login-lab/variants/StepFlow.tsx` (new)
- `web/src/pages/LoginPage.tsx` (refactor to shared hook + lab link)
- `web/src/index.css` (login-lab-drift keyframes)
- `web/src/App.tsx` (route)

## Verification

- `npm run lint`: 0 errors (14 pre-existing warnings, none in new files).
- `npm run build` (tsc -b + vite): passes.
- Headless-browser smoke (vite dev :5199, host google-chrome, playwright):
  `/login-lab` renders title + 5 sidebar variants + preview form; every
  `?v=<id>` renders its form; sidebar click switches preview instantly;
  `/login` still renders + links to the lab. StepFlow advanced correctly:
  invalid identifier shows inline error and stays, valid identifier →
  password step, Back returns, short password blocked with error. End-to-end
  sign-in through SplitPanel with `alice@example.com`/`password123`
  navigated to `/` (full auth flow works). The 500-hit console noise is the
  known pre-existing `/auth/refresh-token` no-cookie 500 (project-notes).

## Reviewer double-checks

- **Visual QA could not be done screenshot-to-eyeball from this session** —
  the variants' aesthetics (spacing, gradients, glass blur) should be
  eyeballed at `http://localhost:5173/login-lab` before picking a keeper.
- `FormItem` is used without a visible `FormLabel` in `CenteredBrand` and
  `Minimal` (label is implied by placeholder) — error text still renders;
  confirm that reads OK.
- The StepFlow advance relies on `form.trigger('identifier')` directly
  rather than `handleSubmit` (see project-notes: handleSubmit only fires
  `onValid` when the whole form is valid). Confirmed working in browser.
- `useLoginFlow` moved the zod schema out of `LoginPage.tsx`; the
  forgot-password `resetSchema` stays local to the page.
- No tests exist for the frontend in this repo (no test runner in
  `web/package.json`) — lint + build + browser smoke are the verification.
---
# SUMMARY — enable-https

HTTPS on port 443 without a domain name (self-signed fallback), plus a switch
that provisions and auto-renews real Let's Encrypt certs the moment a domain
is pointed at the box (no repo changes needed).

## What was changed and why

- `web/Dockerfile`: runtime stage installs `openssl` and removes the stock
  `default.conf`; the http/HTTPS config lives in `web/nginx.conf`. EXPOSE 80 443.
- `web/docker-entrypoint.sh` (new, replaces the stock ENTRYPOINT): before
  starting nginx, writes a 10-year self-signed cert to
  `/etc/letsencrypt/live/gaggle/{fullchain,privkey}.pem` **only if it is
  missing** — so a mounted real cert is never clobbered and the 443 listener
  always boots. Then runs nginx.
- `web/nginx.conf`: added a `listen 443 ssl` server block mirroring :80 (SPA,
  `/api/`, SSE `/api/v1/stream`, `/swagger/`). Both listeners serve the certbot
  ACME webroot at `/.well-known/acme-challenge/` (root `/var/www/certbot`) for
  HTTP-01 validation. :80 keeps serving the app so `http://<ip>` keeps working
  until a domain exists.
- `compose.yaml`: web gains the shared `letsencrypt` (`/etc/letsencrypt`) +
  `certbot_www` (`/var/www/certbot`) volumes and optional
  `"${WEB_HTTPS_PORT:-}:443"` publication (empty = unpublished locally).
- `compose.prod.yaml`: `COOKIE_SECURE: "true"` (prod is HTTPS now); adds a
  `certbot` service that issues `/etc/letsencrypt/live/gaggle` via webroot
  (`--cert-name gaggle`, with `--force-renewal` only when `live/gaggle` is not
  yet certbot-managed) and renews on a 12h loop. Web host ports come from
  `.env` interpolation, so this file does not re-declare ports (Compose merges
  lists by appending, which would duplicate the dev mappings).
- `deploy/apply.sh` + `deploy/.env.production.template` + `.github/workflows/deploy.yml`:
  write `WEB_PORT=80`, `WEB_HTTPS_PORT=443`, optional
  `HTTPS_DOMAIN=${GAGGLE_HTTPS_DOMAIN:-}` to `/srv/gaggle/.env` (new optional
  GitHub secret). Health check now curls `https://localhost/swagger/doc.json -k`.
- `.env.example`, `Makefile`, `README.md`: document `WEB_HTTPS_PORT`,
  `HTTPS_DOMAIN`, the self-signed behavior, and escalation to a real cert.

## Reviewer checkpoints

- certbot `live/gaggle` is a symlink into `archive/gaggle` — the shared volume
  must be mounted at the SAME `/etc/letsencrypt` path in web and certbot, or
  nginx cannot follow the link. Don't "optimize" one side to a subpath.
- The image ENTRYPOINT is now custom (the stock envsubst templating no longer
  runs); nginx conf is static — keep it that way.
- First issuance: the self-signed fallback must be force-replaced or certbot's
  `--keep-until-expiring` would see a fresh 10yr cert and never issue.
- After a certbot renewal the running nginx serves the old cert until the web
  container restarts (documented; every deploy restarts web). Deploy-hook
  reload was considered but the certbot image lacks the docker CLI.
- `COOKIE_SECURE=true` ⇒ the refresh cookie is only stored over HTTPS;
  `http://<ip>` logins will now fail with a secure-cookie warning (intended —
  HTTPS is the migration target).

## Verification

- `docker compose ... config` parses for dev + prod (with and without
  `HTTPS_DOMAIN`); env-driven `WEB_PORT/ WEB_HTTPS_PORT` publish 80+443 in
  prod and leave 443 unpublished locally.
- Built `gaggle-web`; ran it with fresh empty volumes — entrypoint generated the
  cert, nginx booted, https 200 / http 200, `/.well-known/acme-challenge/*`
  served from the webroot over both protocols; restart preserved a
  certbot-style symlinked cert (no regeneration); wiped volume reproduced the
  fallback.
- certbot flag set proven via a `--staging --dry-run` invocation (fails only
  because `example.com` is policy-blocked — external net access not available).
- `go test ./...` all pass; `npm run build` + `npm run lint` (0 errors).
---
# SUMMARY — gaggle-goose-branding

Replaces the placeholder Vite favicon and the sidebar "G" text logo with the
goose-themed Gaggle logo, generated previously from `image.jpg`.

## What was changed and why

- The app had **no real favicon**: `web/index.html` referenced `/vite.svg`,
  but there was no `web/public/` directory, so the browser 404'd. Added
  `web/public/` with raster-rendered assets derived from the transparent
  `goose_max.svg` in `/home/bau/Programming/svg-img`:
  - `web/public/favicon.ico` — transparent goose, 4 ICO frames (16/32/48/64),
    rendered with `rsvg-convert` (librsvg) and packed with ImageMagick
    `magick`. 32KB. (The first pass shipped a 3.6MB SVG trace of the
    orange-tile `image_max.svg`; per reviewer feedback the favicon switched to
    the transparent goose and a real `.ico`.)
  - `web/public/gaggle-goose.png` — transparent goose at 80×80 (2× the 40px
    sidebar slot → crisp on retina), ~6KB. (Replaced a 3.1MB SVG trace that
    the sidebar doesn't need.)
  - The two giant SVG traces (`favicon.svg`, `gaggle-goose.svg`) were deleted
    — masters remain in `/home/bau/Programming/svg-img`. Total payload for
    both icon assets went from ~6.7MB to ~38KB.
- `web/index.html`: favicon `<link>` now points at `/favicon.ico`
  (`type="image/x-icon"`).
- `web/src/layout/SocialMediaLayout.tsx` (App Logo block): dropped the
  primary-colored circle + "G" letter glyph and replaced it with an `<img>`
  of the transparent goose (`w-10 h-10 rounded-full`); the "Gaggle" wordmark
  text is unchanged.

## Files touched

- `web/public/favicon.ico` (new; generated from `goose_max.svg`)
- `web/public/gaggle-goose.png` (new; generated from `goose_max.svg`)
- `web/public/favicon.svg` (removed), `web/public/gaggle-goose.svg` (removed)
- `web/index.html`
- `web/src/layout/SocialMediaLayout.tsx`

## Verification

- Generation: `nix develop /home/bau/Programming/svg-img` +
  `rsvg-convert` + `magick`; ICO frame check via `magick identify`
  (16/32/48/64, sRGB).
- `npm run build` (tsc + vite): passes; both assets emitted into `dist/`.
- `npm run lint`: 0 errors (14 pre-existing warnings, none in
  SocialMediaLayout.tsx).
- Deployed: `docker compose up --build -d web`; `curl localhost:5173`
  serves `favicon.ico` (200, 32KB) and `gaggle-goose.png` (200, 6KB);
  dead SVG paths fall back to index.html (SPA `try_files`).

## Reviewer double-checks

- Visual QA (I could not view rendered images from this session): goose is
  centered in its 1024² canvas with margin, so at 40px the sidebar renders a
  padded, circular-cropped icon; the transparent `.ico` shows the goose on
  both light and dark tabs.
- The favicon is a 32KB ICO — if ever needed bigger (e.g. a PWA manifest),
  consider a 192/512 PNG or re-anchored SVG, but payload was the explicit
  concern.

---

# SUMMARY — auth-validation-sweep (#1–#6)

Repo-wide validation pass requested after the login-min fix. Six issues listed in
the audit were fixed (audit item #7, rune-vs-byte accounting, was folded into #3/#4):
DB limits now enforced rune-aware so over-long payloads get 400s, not Postgres 500s;
client-only rules are now also enforced server-side (or removed if DB doesn't back them).

## What was changed and why

- **#1 Profile PATCH rejected empty optional fields** (`server/internal/models/user.go`):
  `UserProfile.Bio` carried `required`, `Location`/`Website` carried `min=3` — the DB defaults
  these to `''`, so a fresh user could never save a profile edit or clear a field. Removed
  `required`/`min`, kept `max` bounds. Also `Date.UnmarshalJSON` now accepts `""` as "no date
  set" (`server/internal/models/date.go`) — the profile form sends `birth_date: ""` for users
  without one, which used to be a JSON-decode 400. Frontend shows "1-50 characters" for display
  name and its `minLength` dropped 3→1 (`ProfilePage.tsx`).
- **#2 Login caps stricter than registration** (`server/internal/models/auth.go`,
  `web/src/pages/LoginPage.tsx`): login allowed identifier ≤90 / password ≤64 while
  registration allows 96/72. Aligned login to `max=96` / `max=72` on both the backend
  `LoginRequest` tags (which previously had none) and the login zod schema (96/72). Registration's
  `min=8` password floor is NOT mirrored on login (invalid credentials stay a single generic
  message — don't leak validation detail that enables enumeration).
- **#3 Post content >280 chars → 500** (`server/internal/service/post_service.go`):
  `posts.content VARCHAR(280)` had no API check. Added `validateContentLength()` rune-counting
  against 280, called in `Create`, `Update`, and `QuotePost` — over-long content is now a 400.
  Composer textarea gets `maxLength={280}` (or 140 in poll mode) plus a live `n/280` counter
  (`ComposeContent.tsx`); quote dialog textarea capped at 280 (`FeedPost.tsx`; edit dialog already
  was). NOTE: JS `.length` counts UTF-16 units while Go counts runes — the server is the hard stop.
- **#4 Poll question >140 chars → 500** (`server/internal/service/post_service.go`): `validatePoll`
  never checked question length; now rune-checks `≤140` (options already `≤100`, now rune-counted).
- **#5 Username charset was client-only** (`server/internal/models/auth.go`,
  `server/internal/util/json.go`): signup UI allows `[a-zA-Z0-9_]` but the API accepted anything.
  Added `regexp=^[a-zA-Z0-9_]+$` to `RegisterRequest.Username` and registered a `regexp` custom
  validation tag in `util/json.go` (go-playground ships no arbitrary-regex built-in).
- **#6 Bookmark category name uncapped in UI** (`web/src/components/FeedPost.tsx`): the new-category
  input now has `maxLength={50}` (DB backing limit), matching category naming UX.
- **Tests** (`server/internal/handlers/integration_test.go`): added
  `TestProfileUpdateAllowsEmptyOptionalFields`, `TestUsernameCharsetEnforced`,
  `TestPostContentLengthRejected`, `TestPollQuestionLengthRejected` (each would have failed before;
  the poll test surfaced a prep panic: the `regexp` tag wasn't registered).

## Verification
- `go test ./...` — all pass (handlers suite 9.7s).
- `npm run lint` — 0 errors, 14 pre-existing react-refresh warnings.
- `npm run build` — passes (chunk-size warning pre-existing).
- Committed on `agent/auth-validation-consistency` in the `agent-branch/auth-validation-consistency` worktree.

# SUMMARY — auth-validation-consistency

Two auth bugs: the login form rejected 3-character usernames even though signup
allows them (signup min=3 vs login min=4), and registering an already-taken
username/email surfaced only a generic toast, discarding the specific
`USERNAME_EXISTS`/`EMAIL_EXISTS` info the API already returns.

## What was changed and why

- **Login identifier min length aligned to 3** (`web/src/pages/LoginPage.tsx`):
  the login zod schema required `.min(4)` while signup (`SignupPage.tsx`),
  profile editing, and the backend `RegisterRequest` all use min 3. A user who
  registered a 3-char username could never sign in through the form. Changed the
  login schema to `.min(3)`.
- **Backend login min aligned too** (`server/internal/models/auth.go`):
  `LoginRequest.Identifier` was `validate:"required"` only; added `min=3` so the
  API enforces the same floor as registration (defense in depth).
- **Duplicate signup error surfaced** (`web/src/pages/SignupPage.tsx`): the catch
  block swallowed the axios error and toasted a generic message. It now reads
  `error.response.data.error.message` (e.g. "username already exists" /
  "email already exists") from the backend 409 and toasts that, falling back to
  the generic message when the error has no API payload. The backend already
  returned these codes/messages correctly — the bug was purely client-side
  (detection in `server/internal/store/user_store.go:140-153` matches the actual
  index names `unique_username`/`unique_email_case_insensitive` from migration
  `000001`).
- **Regression tests** (`server/internal/handlers/integration_test.go`):
  strengthened `TestAuthFlow` to assert the 409 bodies carry `EMAIL_EXISTS` /
  `USERNAME_EXISTS` codes with non-empty messages, and added
  `TestThreeCharacterUsernameSignIn` proving a 3-char user can register and sign
  in through the API.

## Files touched

- `web/src/pages/LoginPage.tsx`
- `web/src/pages/SignupPage.tsx`
- `server/internal/models/auth.go`
- `server/internal/handlers/integration_test.go`

## Reviewer double-checks

- Frontend build/lint are clean (0 lint errors, 14 pre-existing react-refresh
  warnings); backend `go test ./...` passes.
- Confirm the login form now accepts a 3-char username end-to-end (the backend
  accepted it before too, so no API change beyond the optional `min=3`).
- Confirm a duplicate-username and duplicate-email signup each toast the specific
  backend message, and that a non-conflict failure (e.g. network error) still
  falls back to the generic message.
- Login error handling for *invalid credentials* is intentionally still generic
  ("Login failed, invalid credentials") — account enumeration protection, kept.

---

# SUMMARY — improve-message-flow

Direct-message UX rework: the messages page no longer grows unbounded (it keeps
a fixed height and scrolls the conversation list / message thread), and starting
a new conversation stops auto-sending "Hello!" — picking a user opens an empty
chat UI where the first message is written by the sender.

## What was changed and why

- **Fixed-height message views** (`web/src/layout/SocialMediaLayout.tsx`,
  `web/src/pages/MessagesPage.tsx`, `web/src/pages/ConversationPage.tsx`):
  the main content column is now `h-screen flex flex-col` and the two message
  pages use `flex-1 min-h-0 overflow-y-auto` for their scrollable regions, so
  the page itself never grows beyond the viewport and long threads get an
  internal scrollbar. Feed/other pages are unaffected (they still scroll
  naturally inside the same column).
- **Custom first message** (`web/src/pages/MessagesPage.tsx`): the search
  "pick a user" action previously fired `sendMessage(..., "Hello!")`; it now
  navigates to `/messages/new?user=<username>` and sends nothing.
- **New-conversation route + empty chat UI** (`web/src/App.tsx`,
  `web/src/pages/ConversationPage.tsx`): added `/messages/new`, handled by
  ConversationPage in a "new" mode that shows the target's profile header and an
  empty thread ("You haven't talked to @x yet. Say hello!") with the composer
  active. Sending the first message creates the conversation server-side and
  replaces the URL with the real conversation route. If the user was in search
  results but a conversation already exists, it redirects straight into it.
- **Profile "Message" button** (`web/src/pages/ProfilePage.tsx`): now links to
  `/messages/new?user=...` so first contact opens the empty chat UI instead of
  the old inline pre-filled composer.
- **Blocked users** (`web/src/pages/ConversationPage.tsx`): when the current
  user has blocked the target, the new-conversation composer is disabled with an
  explanatory note. Server-side, the existing `DmService.Send` block check
  (both directions) already rejects messaging blocked users and is covered by
  `TestDMs` in `server/internal/handlers/integration_test.go`.

## Files touched

- `web/src/App.tsx`
- `web/src/layout/SocialMediaLayout.tsx`
- `web/src/pages/MessagesPage.tsx`
- `web/src/pages/ConversationPage.tsx`
- `web/src/pages/ProfilePage.tsx`

## Reviewer double-checks

- Message pages: verify threads/conversation lists scroll internally at both
  mobile and desktop widths and that the fixed-height column doesn't clip
  headers or composer.
- New conversation: from `/messages`, search → click user → empty chat UI →
  type + send → lands on `/messages/:id`; revisiting shows the full history.
- Block UX: messaging someone you've blocked disables the composer with the
  note; messaging someone who blocked you still hits the server 403 (toasted as
  an error).
- No backend/migration changes landed on this branch.

---

# SUMMARY — profile-loading-spinner

The profile page (`/profile/:username`) shows a bare "Loading..." text while the
profile fetch is in flight, instead of the spinning `Loader2` indicator every
other page uses.

## Root cause

`ProfilePage.tsx` is the only page that renders the `profileLoading` state as
inline text (`return <div>Loading...</div>`). Every other data-driven page
(FeedPage, PostPage, HashtagPage, ExplorePage, ConversationPage, the profile
tabs' `ProfileFeedTab`, …) uses a centered `Loader2 className="h-8 w-8
animate-spin text-primary"` spinner. The profile's post/reply/media tab loading
states already used the spinner — only the top-level profile fetch did not.

## Change

`web/src/pages/ProfilePage.tsx`: replaced the plain text with the app-standard
centered spinner, matching the parent column width:

```jsx
if (profileLoading) {
  return (
    <div className="w-full max-w-4xl mx-auto flex items-center justify-center py-20">
      <Loader2 className="h-8 w-8 animate-spin text-primary" />
    </div>
  );
}
```

`Loader2` was already imported. No other behavior changed — the loading
condition, the "Username not found" guard above it, and everything downstream
are untouched.

## Files touched

- `web/src/pages/ProfilePage.tsx`

## Verification

- `npm run lint`: 0 errors (the same 14 pre-existing react-refresh warnings as
  base, none in ProfilePage).
- `npm run build` (tsc + vite): passes.
- Frontend-only; no server, tests, or migrations affected.

## Reviewer double-checks

- Centering/padding: `py-20` provides generous vertical spacing consistent with
  the page's empty/"No posts yet" states; the spinner renders in the same
  `max-w-4xl` column as the profile header so it doesn't drift horizontally on
  narrow/mobile widths.
- `Loader2` already imported at the top of the file — no new dependency added.

---

# SUMMARY — seed-data-strategy

Brainstorm → design spec (no code) for how Gaggle loads dummy data. A fresh
prod deploy currently comes up with an empty DB (the live EC2 site is empty),
because `deploy/apply.sh` auto-migrates on api boot but never runs the seed
binary. This branch records the chosen strategy and the seam for a future
"live users" cron.

## Decisions (resolved via brainstorm questions)

- **Where**: seed runs in the api entrypoint, gated by `SEED_ON_START` (default
  true in compose.yaml), after migrations and before `exec /app/api`. The seed
  is already idempotent (`alice@example.com` guard), so only a fresh DB pays
  the cost; re-deploys no-op. Covers local `make dev` and EC2 prod through one
  switch. Rejected: explicit step in apply.sh (EC2-only, leaves local manual)
  and SQL/psql scripts (unmaintainable, shares no primitives with the cron).
- **Content**: faker library (`brianvoe/gofakeit/v7`, new dep) with a fixed RNG
  seed for determinism. Target: 30 users, ~400 posts + ~150 replies spread over
  the last 28 days, engagement (likes/reposts/bookmarks/polls/mentions/
  hashtags), follows + a few blocks/mutes/private accounts, DMs, lists, badge
  grants, and real (generated PNG) media attached to posts/profiles.
- **Seam for the cron**: new `server/internal/seedgen` package (pure `Generate`
  + DB-writing `Apply`, plus a `Tick` activity-cycle function). `cmd/seed` =
  bulk initial load; future `cmd/simulate` = one activity tick with `now()`
  timestamps, scheduled on the EC2 box via systemd/cron →
  `docker compose run --rm --no-deps --entrypoint /app/simulate api`.
- **Found constraint**: `post_store.go:261` INSERT can't set `created_at`
  (DB-default `now()`). **Fix chosen**: honor `post.CreatedAt` in the store via
  `COALESCE($n, CURRENT_TIMESTAMP)` on both Create INSERTs — app call sites pass a
  zero value → `NULL` → `now()`, unchanged behavior; the seed sets it for
  backdating. No migration/model change needed. See spec §"Backdating posts".

## Files touched

- `docs/superpowers/specs/2026-08-19-seed-strategy-design.md` (new, the spec)

## Reviewer double-checks

- Spec only — no code. The store `created_at` fix choice is resolved (COALESCE
  on both Create INSERTs, §"Backdating posts"); when implementing, confirm the
  two app call sites still pass a zero `CreatedAt`.
- The `cmd/simulate` binary: implement now (prove the seam) or defer — open
  question flagged in the spec.
- No migration was added → no version-collision risk from this branch.
- When implemented, verify a wiped-volume boot seeds automatically and re-boots
  no-op quickly.

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
