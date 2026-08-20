---
description: Merge every finished agent-branch/* branch back into main
agent: build
---

1. Confirm you're in the project root, on the master branch (git checkout master
   if not — do NOT run this from inside an agent-branch worktree).
2. List all worktrees: git worktree list
3. For each ./agent-branch/<slug> directory:
   a. Check it has committed changes (git -C ./agent-branch/<slug> status).
      Skip any with uncommitted changes and report them instead of merging.
   b. Attempt: git merge --no-edit agent/<slug>
   c. If conflicts arise, resolve them using judgment, consulting
      ./agent-branch/<slug>/SUMMARY.md for context on intent.
   d. Append an entry to MERGE_LOG.md: branch, whether clean or conflicted,
      how conflicts were resolved.
   e. Commit the merge.
   f. Remove the worktree: git worktree remove ./agent-branch/<slug>
   g. Tear down the merged branch's isolated preview if one exists:
      docker compose -p gaggle-<slug> down && rm -f ~/.local/state/gaggle-proj/gaggle-<slug>.env
      (both are safe to skip if the preview was never started).
4. At the end, print a summary: which branches merged cleanly, which had
   conflicts, which were skipped (and why).