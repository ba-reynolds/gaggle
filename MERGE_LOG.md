# Merge log

Running log of agent-branch merges into main. Newest on top.

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