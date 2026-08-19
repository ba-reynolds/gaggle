# Merge log

Running log of agent-branch merges into main. Newest on top.

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