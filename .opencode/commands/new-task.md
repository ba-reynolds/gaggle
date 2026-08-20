---
description: Create an isolated worktree and implement a task there
agent: build
---

Task: $ARGUMENTS

0. Check whether the current directory is already inside an agent-branch/*
   worktree (run: git rev-parse --show-toplevel and see if the path contains
   "agent-branch"). If so, STOP and tell the user to run this from the
   project root instead.
1. Come up with a short kebab-case slug for this task.
2. Run: mkdir -p ./agent-branch && git worktree add ./agent-branch/<slug> -b agent/<slug>
3. From this point on, treat ./agent-branch/<slug>/ as your working root.
   Only read, edit, and run commands inside that path — never touch files
   outside it.
4. Implement the task above.
5. Run any available tests/lint inside the worktree.
6. Stand up the isolated preview stack so the user can connect to YOUR build
   (skip only for purely-backend tasks with no visible surface):
   a. Run: make proj-dev   (NOT make dev — that overwrites the shared stack
      every other agent uses).
   b. Wait for it to finish (it passes docker compose --wait, so it only
      returns once every container is healthy). The first run builds the
      api/web images and may take a while.
   c. Copy the "Frontend: http://localhost:<port>" line printed by the script —
      this is YOUR branch's isolated preview, on its own ports and its own DB.
   d. If the UI needs demo users to log in, also run: make proj-seed
7. Write ./agent-branch/<slug>/SUMMARY.md covering: what was changed, why,
   files touched, and anything a reviewer should double-check.
8. Commit everything on that branch (git -C ./agent-branch/<slug> add -A
   && git -C ./agent-branch/<slug> commit -m "<slug>: summary"). The preview
   state (ports, volumes) lives outside the repo under
   ~/.local/state/gaggle-proj/, so nothing preview-related gets committed.
9. Report back to the user:
   - branch name and worktree path
   - the FRONTEND URL to connect to (http://localhost:<port> from step 6c)
   - a one-line summary of the change
   - whether the preview is still running (it is, unless you ran proj-stop)