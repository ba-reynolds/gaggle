---
description: List all active agent-branch worktrees and their state
agent: build
---

1. Run: git worktree list
2. For each entry under ./agent-branch/<slug>:
   - Report the branch name.
   - Run git -C ./agent-branch/<slug> status --short and note whether it's
     clean, has uncommitted changes, or has untracked files.
   - Run git -C ./agent-branch/<slug> log master..HEAD --oneline and note how
      many commits it's ahead of master.
   - If SUMMARY.md exists there, show its first line/title as a one-line
     description of the task.
   - Check whether an isolated preview is running: if
     ~/.local/state/gaggle-proj/gaggle-<slug>.env exists, read its WEB_PORT
     and report the preview URL as http://localhost:<WEB_PORT>.
3. Present as a simple table: branch | status | commits ahead | task summary | preview URL.
4. Do not modify anything — this is read-only.