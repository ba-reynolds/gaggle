# Merge log

Running log of agent-branch merges into main. Newest on top.

## 2026-08-21 — agent/google-oauth

- Status: **clean** (fast-forward from 890d4be → 89cfefd; no conflicts)
- Files merged: Google OAuth end-to-end — backend (`server/pkg/config/config.go` GoogleOAuthConfig, migration `000025_add-google-oauth.{up,down}.sql`, `server/internal/auth/google.go` tokeninfo/code-exchange/userinfo helpers, `auth_service.go` GoogleAuth find-or-link-or-create + random username fallback, `auth_handler.go` GIS credential + code-flow handlers, `router.go` /auth/google/* routes incl. config feature-flag), frontend (`GoogleSignInButton.tsx` proxy-click hidden GIS button, `AuthCallbackPage.tsx`, LoginPage/SignupPage buttons, i18n keys en/es/fr/de for the whole login flow), infra (`compose.yaml` GOOGLE_* env passthrough, `.env.example` docs)
- `SUMMARY.md` auto-merged cleanly (google-oauth section prepended on branch; main had not moved since 890d4be). Migration duplicate check clean: 000025 is unique (prev max 000024). No other parallel branch minted it.
- Live-tested on isolated preview gaggle-google-oauth (:5210/:2110): real Google signup created user `bautistaalvarezr` via GIS credential flow; refresh rotation verified.
- Worktree removed, preview torn down after merge.

## 2026-08-20 — agent/link-click-middle-open (worktree removal — already merged)

- Status: **clean — already up to date** (branch `agent/link-click-middle-open` at 9b536cf is ancestor of HEAD via merge `24ebb35` on 2026-08-20; `git merge --no-edit agent/link-click-middle-open` reports Already up to date, no new changes)
- `git -C ./agent-branch/link-click-middle-open status` clean (no uncommitted changes), worktree removed, preview torn down. Original merge logged on 2026-08-20 as conflicted (SUMMARY.md clobber restored) — no duplicate entry beyond this removal note.

## 2026-08-20 — agent/instant-tabs (worktree removal — already merged)

- Status: **clean — already up to date** (branch `agent/instant-tabs` at e2fb5f8 is ancestor of HEAD via merge `2ac37af` on 2026-08-20; `git merge --no-edit agent/instant-tabs` reports Already up to date, no new changes)
- `git -C ./agent-branch/instant-tabs status` clean (no uncommitted changes), worktree removed, preview torn down. Original merge was logged on 2026-08-20 as conflicted (SUMMARY.md clobber restored) — no duplicate MERGE_LOG entry needed beyond this removal note.

## 2026-08-20 — agent/fix-feed-middle-click

- Status: **clean** (fast-forward from 26a2b17 → e641064; no conflicts)
- Files merged: `SUMMARY.md` (prepended fix-feed-middle-click section above link-click-middle-open/instant-tabs), `web/src/components/FeedPost.tsx` (split handlePostClick/handleAuxClick + handleMouseDown autoscroll suppression + handleKeyDown Enter + focus-visible ring + closest(a,button) guard + auxClick/mouseDown stops on inner controls), `web/src/components/MediaGallery.tsx` (SingleImage auxClick/mouseDown stops), `web/src/components/UserHoverCard.tsx` (a → Link with stopPropagation on click/auxClick/mouseDown)
- `SUMMARY.md` intent consulted: feed middle-click swallowed by autoscroll, keyboard Enter missing, profile click double-push via UserHoverCard bubbling — all preserved as-is (fast-forward, no resolution needed)
- Migration duplicate check clean (no migrations). No DB migrations (frontend only).

## 2026-08-20 — agent/message-page-fixes

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top (message-page-fixes → fix-goose-403-deploy →
    avatar-suggestions-polish → prior sections), `---` separated.
- Real code (`ConversationPage.tsx`, `MessagesPage.tsx`, `index.css` — bounded
  thread height via `h-dvh`, `group-`-prefixed bubble positions, 2px group gap,
  `.theme-scrollbar`) auto-merged cleanly. Migration duplicate check clean. No
  DB migrations (frontend only).

## 2026-08-20 — agent/fix-goose-403-deploy

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top (fix-goose-403-deploy → avatar-suggestions-polish →
    prior sections), `---` separated.
- Real code (`deploy/apply.sh` force-recreate + pre-flight verify + two-protocol
  gate, `compose.yaml` web healthcheck assets) auto-merged cleanly. Migration
  duplicate check clean. No DB migrations (infra only).

## 2026-08-20 — agent/avatar-suggestions-polish

- Status: **clean** (`--no-ff` merge)
- `SUMMARY.md`, `.opencode/project-notes.md` auto-merged cleanly (main had no
  nearby edits at that point). Real code (`UserAvatar.tsx` muted theme-aware
  fallback, `min-w-0`+`truncate` who-to-follow rows in SocialMediaLayout/
  ExplorePage/ListPage, `UserHoverCard.tsx`) auto-merged cleanly. Migration
  duplicate check clean. No DB migrations (frontend only).

## 2026-08-20 — agent/settings-page-bg-split

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top, `---` separated.
- Real code (`web/src/layout/SocialMediaLayout.tsx` sidebar background split)
  auto-merged cleanly. Migration duplicate check clean. No DB migrations.

## 2026-08-20 — agent/settings-appearance-fixes

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top, `---` separated.
- Real code (AppearanceSync, ThemeContext/ThemeToggle/ThemeCustomizer wiring,
  SettingsPage, i18n strings, `web/src/index.css`) auto-merged cleanly.
  Migration duplicate check clean. No DB migrations (frontend only).

## 2026-08-20 — agent/research-db-seeding

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top, `---` separated.
- Research/audit branch (seed admin flag bug, hardcoded seed user IDs) — no code
  changes beyond the log files. Migration duplicate check clean. No DB
  migrations.

## 2026-08-20 — agent/message-grouping

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top, `---`
    separated.
- Real code (`web/src/pages/ConversationPage.tsx` DM bubble grouping,
  `web/src/index.css`) auto-merged cleanly. Migration duplicate check clean.
  No DB migrations (frontend only).

## 2026-08-20 — agent/fix-session-and-static-images

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top, `---` separated.
- Real code (refresh-cookie Secure derives from `X-Forwarded-Proto`,
  same-device token rotation replay handling, deploy/apply.sh `--no-cache` +
  asset health-checks, auth store/service/handler, testutil UA) auto-merged
  cleanly — including `integration_test.go` and `store.go` which admin-metrics
  and avatar branches had also touched. Migration duplicate check clean. No DB
  migrations.

## 2026-08-20 — agent/avatar-placeholder

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top, `---` separated.
- Real code (new `UserAvatar.tsx` shared component, 13 call sites swapped off
  copy-pasted Avatar blocks) auto-merged cleanly. Migration duplicate check
  clean. No DB migrations (frontend only).

## 2026-08-20 — agent/admin-metrics-panel

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top, `---` separated.
- Real code (metrics pkg, visit middleware, metrics store/service wiring,
  `/admin/metrics` + `/admin/metrics/history` endpoints, swagger, sampler,
  MetricsDashboard, useAdmin, types/api) auto-merged cleanly. `compose.yaml`
  merged cleanly (branch's `- /:/host:ro` volume + main's `API_PORT` /
  container_name removals / named cache volumes all preserved).
- Migrations: branch adds `000022_create-page-views` + `000023_create-host-`
  `metrics-samples`, both next free slots — pre- and post-merge duplicate
  checks clean, no renumbering.

## 2026-08-19 — agent/snappy-ux

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (snappy-ux → nickname-default follow-up → nickname-default →
    web-healthcheck → grok-logo → f5-loading-ui → news-preview → prior
    sections), `---` separated.
  - `.opencode/project-notes.md` — kept **every** section from both sides,
    newest on top (snappy-ux → birth-date sentinel → nickname-default →
    grok-logo → f5-loading-ui → base), `---` separated.
- Real code (useSendMessage optimistic DMs, global staleTime 60s, prefetch in
  SocialMediaLayout, `Message.pending?`, shared useNotifications hook, SSE
  `['dm-messages']` invalidation) **auto-merged cleanly** — including
  `SocialMediaLayout.tsx` which f5-loading-ui had also touched. Migration
  duplicate check clean. No DB migrations added.

## 2026-08-19 — agent/nickname-default

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (nickname-default follow-up → nickname-default → web-healthcheck →
    grok-logo → f5-loading-ui → news-preview → prior sections), `---`
    separated.
  - `.opencode/project-notes.md` — kept **every** section from both sides,
    newest on top (birth-date sentinel → nickname-default → grok-logo →
    f5-loading-ui → base), `---` separated.
- Real code (`user_store.go` display_name default, `integration_test.go`
  `TestProfileLifecycle`, `ProfilePage.tsx` birthday sentinel) auto-merged
  cleanly. Migration duplicate check clean. No DB migrations added.

## 2026-08-19 — agent/web-healthcheck

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (web-healthcheck → grok-logo → f5-loading-ui → news-preview → prior
    sections), `---` separated.
- Real code (`compose.yaml` web healthcheck, `deploy/apply.sh` `up --wait`)
  auto-merged cleanly. Migration duplicate check clean. No DB migrations.

## 2026-08-19 — agent/grok-logo

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (grok-logo → f5-loading-ui → news-preview → prior sections), `---`
    separated.
  - `.opencode/project-notes.md` — kept **every** section from both sides,
    newest on top (grok-logo → f5-loading-ui → base), `---` separated.
- Real code (`web/public/gaggle-goose.png` / `favicon.ico` binary swaps — no
  code changed) merged cleanly. Migration duplicate check clean. No DB
  migrations.

## 2026-08-19 — agent/f5-loading-ui

- Status: **clean**
- Auto-merged cleanly (`AppSplash.tsx` boot splash, pre-JS splash in
  `index.html`, AuthContext bootstrap timeout). Migration duplicate check
  clean. No DB migrations.

## 2026-08-19 — agent/news-preview

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (news-preview → settings-language → message-conversation-fixes →
    login-experiments follow-ups → enable-https → prior sections), `---`
    separated.
- Real code (linkmeta package, news_store, post/news handlers, list/search/
  post services, ComposeContent/FeedPost/PollCard, router, docs/swagger,
  go.mod, package-lock) **auto-merged cleanly** — including
  `integration_test.go`, `service.go`, `store.go`, `ComposeContent.tsx`,
  `FeedPost.tsx`, `types/api.ts` which both settings-language and this branch
  had touched.
- Migrations: branch brings `000021_add-post-news.{up,down}.sql`. Pre-merge
  duplicate check on `main` (versions ≤ 000020) clean; post-merge check clean.
  No collision, no renumbering needed.

## 2026-08-19 — agent/settings-language

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (settings-language → message-conversation-fixes → login-experiments
    follow-ups → enable-https → prior sections), `---` separated.
  - `web/src/App.tsx` — settings-language added the `I18nProvider` wrapper;
    login-experiments had added the `/login-lab` route. Kept BOTH: the
    provider wraps the Router and the `/login-lab` route is preserved.
  - `web/src/pages/LoginPage.tsx` — genuine design collision: login-experiments
    promoted the StepFlow variant on `/login`, while settings-language (branched
    earlier) reworked the old card page with i18n. Resolution: kept the
    **promoted StepFlow design** and applied the branch's **i18n `t()` labels**
    (auth.* keys) to the footer, reset card, and copy. Also routed
    `useLoginFlow`'s toasts through `t("auth.loginSuccess"/"auth.loginFailed")`
    so localized behavior is preserved on the honored design.
  - `.opencode/project-notes.md` auto-merged cleanly.
- Real code (I18nContext, i18n/*.ts, SettingsPage/SignupPage/UserHoverCard/
  AuthContext/useSettings/store/user_store/auth_service & handler) auto-merged
  cleanly. Migration duplicate check clean. No DB migrations added.

## 2026-08-19 — agent/message-conversation-fixes

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (message-conversation-fixes → login-experiments follow-ups →
    enable-https → prior sections), `---` separated.
  - `.opencode/project-notes.md` — kept **every** section from both sides,
    newest on top (Messages debounce/new-route → Login lab → enable-https →
    promotion entry → base), `---` separated.
- Real code (ConversationPage new-route fix, MessagesPage/ListPage/ExplorePage
  debounce, useDebounce SEARCH_DEBOUNCE_MS, AdminPage/BookmarksPage) merged
  cleanly. Migration duplicate check clean. No DB migrations (web only).

## 2026-08-19 — agent/login-experiments

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (login-experiments follow-ups → enable-https → prior sections), `---`
    separated.
  - `.opencode/project-notes.md` — kept **every** section from both sides,
    newest on top (Login lab/useLoginFlow → enable-https → promotion entry →
    base), `---` separated.
- Real code (useLoginFlow hook, login-lab variants, LoginPage.tsx rewrite,
  App.tsx, index.css, theme-themes.css) auto-merged cleanly. Migration
  duplicate check clean. No DB migrations on this branch (web only).

## 2026-08-19 — agent/enable-https

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `.opencode/project-notes.md` — kept **every** section from both sides,
    newest on top (enable-https → login-experiments → base), `---` separated.
- `SUMMARY.md` auto-merged cleanly; real code (compose, nginx, Dockerfile,
  deploy scripts, workflows, README, Makefile, `.env.example`) auto-merged
  cleanly. Migration duplicate check clean after merge. No DB migrations on
  this branch (infra only).

## 2026-08-19 — agent/seed-data-strategy

- Status: **conflicted** (spec-only branch, no code)
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (all prior sections → seed-data-strategy → base), `---` separated.
  - `.opencode/project-notes.md` — kept **every** section from both sides,
    newest on top (goose branding, auth-validation, seed backdating, then base).
- No real code conflicts (`docs/superpowers/specs/2026-08-19-seed-strategy-design.md`
  is the only new file). Migration duplicate check clean after merge. No
  migration added on this branch.

## 2026-08-19 — agent/profile-loading-spinner

- Status: **conflicted**
- File conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (all prior sections → profile-loading-spinner → base), `---` separated.
- `.opencode/project-notes.md` auto-merged cleanly; real code
  (`web/src/pages/ProfilePage.tsx`) auto-merged cleanly. Migration duplicate
  check clean after merge. Frontend-only; no server/DB changes.

## 2026-08-19 — agent/improve-message-flow

- Status: **conflicted**
- File conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (gaggle-goose-branding → auth-validation sections → improve-message-flow
    → base sections), `---` separated.
- `.opencode/project-notes.md` auto-merged cleanly; real code
  (`web/src/pages/MessagesPage|ConversationPage|ProfilePage.tsx`, App.tsx,
  SocialMediaLayout.tsx) auto-merged cleanly. Migration duplicate check clean
  after merge. No server/DB changes.

## 2026-08-19 — agent/auth-validation-consistency

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (gaggle-goose-branding → auth-validation-sweep #1–#6 →
    auth-validation-consistency → base sections), `---` separated.
  - `.opencode/project-notes.md` — kept **every** section from both sides,
    newest on top (goose branding, then auth-validation sweep, then base).
- Real code (`server/` auth/models/service + `integration_test.go`,
  `web/src/pages`/components) auto-merged cleanly. Migration duplicate check
  clean after merge.

## 2026-08-19 — agent/gaggle-goose-branding

- Status: **clean** (`--no-ff` merge)
- `.opencode/project-notes.md`, `SUMMARY.md` auto-merged cleanly (main had no
  nearby edits). `web/` assets (favicon.ico, goos logo) and
  `web/src/layout/SocialMediaLayout.tsx` merged cleanly. Migration duplicate
  check clean after merge. Frontend-only; no server/DB changes.

## 2026-08-19 — agent/sidebar-mobile-nav

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top
    (chat-ui-fixes first, then sidebar-mobile-nav, then the base sections).
- `.opencode/project-notes.md`, `web/src/components/MobileNavigation.tsx`,
  `web/src/layout/SocialMediaLayout.tsx` auto-merged cleanly. Migration
  duplicate check clean after merge. Frontend-only; no server/DB changes.

## 2026-08-19 — agent/chat-ui-fixes

- Status: **clean** (`--no-ff` merge)
- `SUMMARY.md` came in on the branch prepended; merged cleanly (main had no
  nearby edits). `web/src/components/PollCard.tsx`,
  `web/src/pages/ConversationPage.tsx`, `web/src/pages/ProfilePage.tsx`
  auto-merged cleanly. Migration duplicate check clean after merge.

## 2026-08-18 — agent/trim-theme-catalog

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top.
- `web/src/contexts/ThemeContext.tsx`, `theme-themes.css`, `ThemeCustomizer.tsx`
  auto-merged cleanly. Migration duplicate check clean after merge.

## 2026-08-18 — agent/profile-action-buttons-align

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top.
- `web/src/pages/ProfilePage.tsx` auto-merged cleanly. Migration duplicate check
  clean after merge.

## 2026-08-18 — agent/poll-question-trending

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top.
  (This merge surfaced that the earlier `fuzzy-search-results` merge had left
  stray conflict markers in `SUMMARY.md` and dropped the base sections; fixed
  afterward by reconstructing `SUMMARY.md` from branch sources — see the
  `fuzzy-search-results` entry below.)
- `server/internal/handlers/integration_test.go`, `post_service.go`,
  `web/src/components/ComposeContent.tsx` auto-merged cleanly. Migration
  duplicate check clean after merge.

## 2026-08-18 — agent/fuzzy-search-results

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top.
  - NOTE: the branch's `SUMMARY.md` contained only its own section (base
    stripped), and the first-pass resolution left stray conflict markers +
    dropped the base sections. Reconstructed `SUMMARY.md` from branch sources
    (poll, fuzzy, bookmark, account sections on top + the intact pre-merge
    base), keeping every section, newest on top.
- `.opencode/project-notes.md` and `post_store.go`/`integration_test.go`
  auto-merged cleanly. Migration duplicate check clean after merge.

## 2026-08-18 — agent/fix-bookmark-like-increments-view

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section from both sides, newest on top.
- `server/internal/handlers/integration_test.go` (both sides' tests retained),
  `server/internal/store/post_engagement_store.go` auto-merged cleanly.
  Migration `000019_dedupe-post-views` merged cleanly; duplicate check clean.

## 2026-08-18 — agent/account-and-post-privacy

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top.
  - `server/cmd/migrate/migrations/000017_add-refresh-token-session.{up,down}.sql`
    — rename/delete (main had renamed the migration to `000017`; the branch had
    the redundant pre-rename `000016` copy deleted on the branch). Kept main's
    `000017` version (content identical), branch's deletion honored.
- **Migration renumber before merge**: branch added `000017_account-post-privacy`
  colliding with main's `000017_add-refresh-token-session`. In the branch
  worktree, dropped the redundant `000016_add-refresh-token-session` copy and
  renumbered `000017_account-post-privacy` → `000020_account-post-privacy`
  (`807bc72`) before merging. Migration duplicate check clean after merge.

## 2026-08-18 — agent/move-themes-to-settings

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top.
- `web/src/layout/SocialMediaLayout.tsx`, `ThemeCustomizer.tsx`,
  `ThemeContext.tsx`, `theme-themes.css` auto-merged cleanly.

## 2026-08-18 — agent/message-gradient

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top.
- `web/src/index.css`, `web/src/pages/ConversationPage.tsx` auto-merged cleanly.

## 2026-08-18 — agent/detailed-search-filters

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top.
  - `server/internal/handlers/integration_test.go` — both sides added tests
    (`TestMentionsFeed` from main, `TestSearchFilters` from the branch); kept
    **both** test functions.
  - `web/src/hooks/useSearch.ts` — combined both import lists (`getMentionsFeed`
    retained, `type PostSearchFilters` added).
- Migration duplicate check clean after merge.

## 2026-08-18 — agent/pin-unpin-timeline-bug

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — kept **every** section, newest on top.
- Migration `000016_add-refresh-token-session` → `000017` rename merged cleanly
  (identical to the profile-relationships-view fix, already on main);
  `web/src/hooks/usePost.ts` auto-merged (optimistic `is_pinned` update).

## 2026-08-18 — agent/tag-users-in-posts

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md`, `.opencode/project-notes.md` — kept **every** section from both
    sides, newest on top.
- **Migration renumber before merge**: branch added `000017_add-post-mentions`,
  colliding with `000017_add-refresh-token-session` (renamed in main). Renumbered
  to `000018_add-post-mentions` on the branch (`e039ae1`) before merging, so the
  merged tree has unique versions and the api boots (verified: applies to v18).

## 2026-08-18 — agent/profile-relationships-view

- Status: **clean** (fast-forward)
- Only substantive change vs main: `000016_add-refresh-token-session` →
  `000017_add-refresh-token-session` rename, fixing the duplicate-000016 state
  that blocked api startup (`main` previously had two `000016_*` families).

## 2026-08-18 — agent/refresh-token-rotation

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — both sides prepended their own section; kept **every** section,
    newest on top, separated by `---`.

## 2026-08-18 — agent/post-thread-and-bookmark-fixes

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — both sides prepended their own section; kept **every** section,
    newest on top.
  - `.opencode/project-notes.md` — both sides prepended notes sections; kept **every**
    section from both sides plus the shared `Server: home feed Redis cache` /
    `Theme system` context, newest on top.

## 2026-08-18 — agent/pinned-post-menu-fixes

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — both sides prepended their own section; kept **every** section,
    newest on top.
  - `.opencode/project-notes.md` — both sides prepended notes sections; kept **every**
    section from both sides, newest on top.
- `server/internal/handlers/post_handler.go` auto-merged cleanly.

## 2026-08-18 — agent/google-oauth-analysis

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — both sides prepended their own section; kept **every** section,
    newest on top.

## 2026-08-18 — agent/fix-profile-tabs-and-user-relations

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — both sides prepended their own section; kept **every** section,
    newest on top.

## 2026-08-18 — agent/cloud-deploy-email-analysis

- Status: **clean**

## 2026-08-18 — agent/ui-responsiveness-fixes

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `.opencode/project-notes.md` — both sides added/rewrote sections (agent's
    rewrite dropped ~130 lines of inherited notes per git's delete-when-other-side
    untouched rule). Reconstructed to keep **every** section from both sides:
    main's full notes plus the agent's `Dev stack & parallel agents`, `React Query
    cache gotchas`, and `Server: parent-chain hydration` sections, newest on top.
  - `web/src/components/FeedPost.tsx` — trivial operand reorder
    (`isOwnPost && post.edited_at` vs `post.edited_at && isOwnPost`); logically
    identical, kept the agent branch's ordering.
- Verified after merge: `go test ./...`, `npm run build`, and `npm run lint`
  all pass (lint: 0 errors).

## 2026-08-20 — agent/instant-tabs

- Status: **conflicted**
- Files conflicting, and how resolved:
  - `SUMMARY.md` — branch had 22-line standalone section, auto-merge dropped the 3723-line running log. Restored by prepending branch section above main's log (`---` separated), keeping every section newest on top. Amended merge commit `cc9d363`.
  - `web/src/App.tsx` (per-route Suspense + RouteFallback), `web/src/layout/SocialMediaLayout.tsx` (NavItem preload, idle warm, trends warm) auto-merged cleanly.
- Migration duplicate check clean (no migrations). Verified `npm run build` + `tsc --noEmit` + `npm run lint` (0 errors, 17 warnings pre-existing) post-merge.

## 2026-08-20 — agent/link-click-middle-open

- Status: **conflicted** (SUMMARY.md clobber; resolved to keep both)
- Files conflicting, and how resolved:
  - `SUMMARY.md` — auto-merge kept only the incoming 25-line file (clobber of instant-tabs 3747-line file). Fixed by concatenating: new `link-click-middle-open` section on top (`---` separated) then existing `instant-tabs` + prior sections. Produced 3775-line combined file.
  - `.opencode/project-notes.md` — identical on both sides; no merge needed.
- Real code (`web/src/components/ContentLinks.tsx` URL linkify + splitUrlToken + auxClick, `web/src/components/FeedPost.tsx` middle-click onAuxClick) merged cleanly. Seed/bookmark 4× work (000024 trigger drop + Uncategorized filter) on main was orthogonal — no code conflicts.
- Migration duplicate check clean (`git ls-tree` — no `DUP`). Code `000024` is ours; link branch had no migrations.
- .superpowers/sdd workspace + plan doc not merged (gitignored/untracked plan).
