## Areas

### Working directory

The **working directory** is where you perform actual development. It is the *checked-out* version of the project files—what you see and edit in your file system. When you clone a repository or switch branches, Git populates the working directory with the appropriate version of the files. Any modifications you make (e.g., editing files, adding new ones, or deleting existing ones) are initially reflected only in the working directory and are untracked by Git until further action is taken.

### Staging area

The **staging area**, also referred to as the index, is an intermediate space where changes are prepared before being committed. When you run `git add`, you're moving changes from the working directory into the staging area. This allows you to craft commits intentionally by selecting specific changes rather than committing everything that has changed. 

### Repository

The *repository*, specifically the `.git` directory, is the actual database where Git stores the project history. When you commit changes using git commit, Git records a snapshot of the contents of the staging area into the repository as a new commit object, updating pointers such as `HEAD` to reflect the latest state.

### Normal workflow

Suppose that you are on the `mater` branch and you've just made a commit, let's analyze how each area behaves as we use git.

Initial state:
 - **Working directory**: Exactly matches the contents of the latest commit on the master branch. All files are clean, no modifications.
 - **Staging area**: Contains an index reflecting the exact state of the last commit.
 - **Repository**: Contains the latest commit on master. The HEAD pointer references this commit, which represents the current project state.


You edit files, add new files, or delete files in your working directory:
 - **Working directory**: Now contains changes that differ from the staged snapshot (which still reflects the last commit).
 - **Staging** area: Remains unchanged, still reflecting the last committed snapshot.
 - **Repository**: Unchanged, still pointing to the last commit.

You run `git add <file>` to update the staging area:
 - **Working directory**: Still contains your edits.
 - **Staging area**: Updated with snapshots of the staged files at their current state in the working directory.
 - **Repository**: Still unchanged, pointing to the previous commit.

Suppose you further edit files after staging:
 - **Working directory**: Contains new modifications not yet staged.
 - **Staging area**: Contains the previously staged snapshot, not reflecting the newest changes.
 - **Repository**: Unchanged.

You run `git commit -m "Commit message"`:
 - **Working directory**: Remains as is, including the unstaged changes we've made in the previous step.
 - **Staging area**: Resets to mirror the new commit snapshot after the commit completes.
 - **Repository**: Updated to include the new commit, with HEAD pointing to it.

At this point, the staging area will be the exact same as the repository, the working directory will contain those changes we've made after the initial staging.

### Difference in areas

#### Working area is different from staging area

This means that you've made changes to files in your working directory, but those changes have **not** been added to the staging area using `git add`.


If you run `git diff`, it will show differences between the working directory and the staging area. These changes are not ready to be committed yet. They are untracked by Git as far as the next commit is concerned.

#### Staging area is different from git repository

This occurs when you've staged changes using `git add`, but you have **not** yet committed them with `git commit`.

In this case, `git diff --cached` (or `git diff --staged`) will show the differences between the staging area and the last commit (HEAD). This indicates what *will* be committed if you run `git commit`.

#### Working area is the same as staging area

This means there are **no changes in the working directory** that haven't been staged. In other words, what's in your working directory has already been added to the staging area.

If `git diff` produces no output, this is the case. However, the staging area may still differ from the repository if those staged changes haven't been committed yet.

#### Staging area is the same as repository

This happens after you've committed all your staged changes, and you haven't staged anything new since then. The staging area mirrors the latest commit (HEAD).

Running `git diff --cached` yields no output, meaning there are no pending changes to be committed. However, your working directory might still contain changes not yet staged.

#### Everything is the same

This is the cleanest state. The working directory matches the staging area, and the staging area matches the repository. In this state:
* `git diff` returns nothing (no changes in working directory),
* `git diff --cached` returns nothing (no staged changes),
* `git status` reports "nothing to commit, working tree clean".

No modifications exist anywhere in the project—everything is committed and synchronized.

#### Everything is different

This means that:
* The working directory contains changes that have not been staged.
* The staging area contains changes that have not been committed.
* The repository (HEAD) is out of sync with both.

Here, `git diff` will show the difference between the working directory and staging area, and `git diff --cached` will show differences between the staging area and the repository. This situation can arise during active development, where some changes have been staged and others haven't, and nothing has yet been committed.

## Concepts

### Tracked and untracked files

**Tracked files** are those that Git already knows about. These files have either been committed at least once or have been explicitly staged via `git add`. Tracked files are further subdivided into **three states**:
 - **Unmodified**: a file that has been committed.
 - **Modified**: a file that has been edited since the last commit.
 - **Staged**: it has been modified and added to the index (staging area)

**Untracked files**, on the other hand, are those present in the working directory that Git has not been told to track. These files have never been staged or committed and thus aren't part of the project history.

### `HEAD`

In Git, `HEAD` is a fundamental concept representing the current position or reference point in your repository’s commit history. More technically, `HEAD` is a symbolic reference that points to the tip of the current branch you have checked out in your working directory.

When you check out a branch, Git sets `HEAD` to point to that branch’s latest commit. For example, if you’re on the branch named `main`, `HEAD` points to `main`, which in turn points to the latest commit on that branch. This means that `HEAD` indirectly points to the commit you’re currently working on.

`HEAD` can also point directly to a specific commit in what’s called a **detached HEAD** state. This happens when you check out a commit by its SHA hash or a tag rather than a branch name. In this state, `HEAD` no longer points to a branch but directly to a commit, and any new commits you make won’t belong to any branch unless you explicitly create a new branch from that point.

You'll often see `HEAD` notation such as `HEAD~1`, this means "1 commit previous to the one `HEAD` currently points to".

### Branches

Git is a version control system where everything is built around commits. A commit is a snapshot of your entire project at a given point in time. Each commit has a unique SHA-1 hash ID and a reference to its parent commit(s), forming a chain—a commit graph.

Now, a branch in Git is simply a label pointing to a specific commit. Think of it as a named pointer. Whenever you want to create a branch, you have to visit the branch from which you want to branch off.

Here's a technical breakdown:
1. Each branch is stored as a file in `.git/refs/heads/`. The content of that file is just a SHA-1 hash pointing to the latest commit on that branch.
2. When you create a new branch, Git creates a new file in `.git/refs/heads/` with the same SHA as your current commit, effectively creating a second pointer to the same place.
3. When you switch to that branch (with `git checkout` or `git switch`), Git updates `HEAD` to point to the new branch.
4. If you commit while on this new branch, the commit graph grows from that point and the new branch pointer moves forward, while the old branch remains where it was—until you explicitly switch back and commit from there.

#### Common workflow

Suppose that you are working on a project entirely on the `master` branch so far, you decide that you want to add a new feature so you create a new branch called `feature/user-auth`.


1. We first make sure we are in the `master` branch by visiting it with `git switch master`.
2. We create and visit the new feature branch with `git switch -c feature/user-auth`.
3. We modify the files to add the new feature, we add them to the staging area with `git add .` and finally we commit the changes with `git commit -m "Implement login functionality"`.
4. With the feature already in place in the `feature/user-auth` branch, we want to merge it into `master`, to do this we switch onto `master` with `git switch master` and then merge it with `git merge feature/user-auth -m "Add user authentication feature onto master branch"`. Do note that this will create an additional commit.
5. Finally, we can delete the branch now that it is no longer useful to us with `git branch -d feature/user-auth` - this will delete the branch file in `.git/refs/heads/feature/user-auth`

```
# Create `feature/user-auth` branch from commit `C`
A---B---C (main)
         \
          D (feature/user-auth)


# Merge branch back to master, this creates commit `E`
A---B---C-----------E (main)
         \         /
          D-------   (feature/user-auth)
```

## Commands

### `git reset`

Explanation video: https://www.youtube.com/watch?v=WqIo4dz1JcM

`git reset` is used to undo changes by moving the current HEAD and optionally updating the staging area (index) and working directory. It essentially lets you "rewind" your repository to a previous state.

At a fundamental level, `git reset` changes which commit your current branch reference (like `master` or `main`) points to, effectively telling Git, "this commit is now the tip of the branch".

`git reset` comes in three different flavours:
 - `--hard`: updates the working and staging area to match the specified commit
 - `--mixed`: updates just the staging area to match the specified commit
 - `--soft`: doesn't update the working and staging area

When moving the tip of the branch backwards with `git reset`, the commits that are past the new tip still exist, but they are unreachable because no branch points to them anymore. You can still go back to them with `git reflog`.

One common use case for `git reset --mixed` is in case you make a commit, forgot to add something so you `reset --mixed` (which will keep everything in the working directory), modify the files, add them to the staging area and then commit.

### `git revert`

Explanation video: https://www.youtube.com/watch?v=XJqQPNudPSY

`git revert` is a command used to undo changes in a Git repository by creating a new commit that reverses the effects of a previous commit. 

Unlike commands such as `git reset`, which can rewrite history by moving the current branch pointer backward, `git revert` is a safe way to undo changes in a public or shared history because it does not remove commits but rather adds a new commit that negates the changes introduced by an earlier one.

When you run `git revert <commit>`, Git looks at the specified commit, calculates the inverse of the changes it introduced, and applies those inverse changes on top of the current HEAD. This creates a new commit that effectively “undoes” the previous commit without modifying the commit history.

The original commit remains intact and visible in the project’s history, which is important for maintaining transparency and auditability in collaborative environments.

Git revert is often used when you want to back out a specific commit that has already been pushed and shared with others, ensuring that everyone's history remains consistent.

### `git restore`

The primary function of `git restore` is to restore file contents to a previous state, either from the index (staging area) or from a commit (typically `HEAD`).

When you run `git restore <path>`, Git replaces the content of the file in the working directory with the version in the index. This is effectively discarding unstaged changes to the file, reverting it to its last staged state.

You can also instruct Git to unstage changes by adding the `--staged` flag:
```
git restore --staged foo.c
```

This will copy the content of `foo.c` from `HEAD` (or a specified commit) into the index, removing it from the staged set.

`git restore` is useful in cases where:
1. You accidentaly stage a file you didn't mean to.
2. You've made some changes to a file that you regret, so you want to restore the contents of the file to what they were in the latest commit.


### `git switch`

`git switch <branch-name>` allows you to switch to an already existing branch, alternatively you can use the `-c` flag to create and switch to the newly created branch.

Similar behavior can be achieved with `git checkout <branch-name>`.

### `git cherry-pick`

`git cherry-pick` is used to apply the changes introduced by one or more existing commits from another branch or from elsewhere in your Git repository history, onto the current branch you have checked out.

Technically, what happens during a cherry-pick is that Git takes the patch (the difference introduced) from the specified commit(s) and attempts to replay or apply it as a new commit on top of your current HEAD. This operation does not move branches or change the commit history of the source branch but rather creates new commits in your current branch with the same changes.

One common use-case is for when you apply a commit onto the wrong branch (e.g. `master` instead of `feature/user-auth`), a simple way to fix it is as follows:
1. Use `git log` while on `master` to get the hash of the misplaced commit.
2. Switch to the `feature/user-auth` branch.
3. Run `git cherry-pick <commit-hash>` to apply the commit in this branch
4. Switch back to `master`
5. Run `git reset --hard HEAD~1`

### `git stash`

The `git stash` command in Git is used to temporarily save changes in your working directory that you do not want to commit yet, allowing you to switch contexts (such as checking out another branch) without losing your current modifications. Do keep in mind that when stashing, the working directory will change to match the commit you are currently on.

By default, untracked and ignored files are not included unless explicitly requested via options such as `-u` or `--all`.

Common variations of `git stash`:
* `git stash`: saves both staged and unstaged changes to a new stash entry and resets the working directory to match `HEAD`. By default, does not include untracked or ignored files.
* `git stash -u`: stashes staged, unstaged, and untracked files.
* `git stash -a`: stashes everything, including ignored files.
* `git stash apply`: reapplies the most recent stash (`stash@{0}` by default) without removing it from the stash stack.
* `git stash pop`: reapplies the most recent stash and removes it from the stack.
* `git stash list`: shows all stash entries in the form `stash@{n}: <message>`.
* `git stash drop stash@{n}`: Removes a specific stash entry.
* `git stash clear`: Deletes all stash entries.

---

Todo:
- checkout
- restore
- switch