# Merge log

Running log of agent-branch merges into main. Newest on top.

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