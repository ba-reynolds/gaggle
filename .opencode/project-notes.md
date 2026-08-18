# Project notes — GopherSocial

Tricky/surprising things worth remembering. Newest on top.

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
