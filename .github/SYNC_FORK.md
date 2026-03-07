# Step-by-Step Guide to Sync Your Fork

This guide explains how to sync your fork of rclone with the original rclone repository to get the latest changes.

## Prerequisites

- You have a fork of rclone (e.g., `https://github.com/Breschling/rclone`)
- You have cloned your fork locally
- You have made changes to your fork (e.g., added the level3 backend)

## Step 1: Add the Upstream Remote

First, add the original rclone repository as an "upstream" remote:

cd /Users/hfischer/go/src/rclone
git remote add upstream https://github.com/rclone/rclone.git**Verify it was added:**sh
git remote -vYou should see:
- `origin` → your fork (Breschling/rclone)
- `upstream` → original repo (rclone/rclone)

**Note:** If you get an error saying the remote already exists, you can skip this step or update it with:h
git remote set-url upstream https://github.com/rclone/rclone.git## Step 2: Fetch Latest Changes from Upstream

Fetch all branches and commits from the original repository:
sh
git fetch upstreamThis downloads the latest commits from upstream without modifying your local branches.

## Step 3: Check Your Current Branch

Make sure you're on the branch you want to update (likely `master`):

git branchIf you're not on `master`, switch to it:
git checkout master## Step 4: Merge Upstream Changes

Merge the upstream `master` branch into your local `master` branch:

git merge upstream/master**If there are conflicts:**
- Git will pause and mark the conflicted files
- Open the conflicted files and resolve the conflicts manually
- After resolving, stage the files: `git add <file>`
- Complete the merge: `git commit`

Since you only added the level3 backend, conflicts are unlikely. If conflicts do occur, they will be in files that were modified both upstream and in your fork.

## Step 5: Push to Your Fork

Push the updated `master` branch to your fork:

git push origin master## Alternative: Using Rebase (Optional)

If you prefer a linear history instead of merge commits, you can use rebase instead:

git rebase upstream/master**If there are conflicts during rebase:**
1. Resolve the conflicts in the files
2. Stage the resolved files: `git add <file>`
3. Continue the rebase: `git rebase --continue`

**Then force-push** (since rebase rewrites history):sh
git push origin master --force-with-lease⚠️ **Warning:** Only use force-push if you're the only one working on this branch. The `--force-with-lease` flag is safer than `--force` as it will prevent you from overwriting changes you haven't fetched.

## Quick Reference: Future Updates

For future updates, you only need to repeat steps 2, 4, and 5:

git fetch upstream
git merge upstream/master
git push origin masterOr create a Git alias for convenience:
git config alias.sync-upstream '!git fetch upstream && git merge upstream/master'Then you can simply run:
git sync-upstream## Important Notes

1. **Backup First:** Always commit or stash your current work before syncing
2. **Conflicts:** Since you only added the level3 backend, conflicts are unlikely, but always review changes carefully
3. **Test After Syncing:** After syncing, test your level3 backend to ensure it still works with the updated codebase
4. **Branch Protection:** If your fork has branch protection enabled, you may need to push via a pull request instead

## Troubleshooting

### "Remote upstream already exists"
If you see this error, the upstream remote is already configured. You can verify with `git remote -v` and proceed to Step 2.

### "Your branch is ahead of 'origin/master'"
This is normal after merging. Just push your changes with `git push origin master`.

### Merge conflicts
If you encounter conflicts:
1. Git will show which files have conflicts
2. Open each file and look for conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`)
3. Edit the file to resolve the conflict
4. Stage the file: `git add <file>`
5. Continue: `git commit` (for merge) or `git rebase --continue` (for rebase)
