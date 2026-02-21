# Git Workflow: Syncing Fork with Official rclone Upstream

## Overview

This document describes the process for pulling updates from the official rclone upstream repository (`https://github.com/rclone/rclone`) into this fork to keep it synchronized with the latest changes.

## Step-by-Step Guide

### 1. Check Current Remotes

First, verify what remotes are currently configured:

```bash
git remote -v
```

You should see at least `origin` (your fork). If you don't see `upstream`, you'll need to add it.

### 2. Add Upstream Remote (if not already added)

If the upstream remote doesn't exist, add it:

```bash
git remote add upstream https://github.com/rclone/rclone.git
```

Or if you prefer SSH:

```bash
git remote add upstream git@github.com:rclone/rclone.git
```

### 3. Fetch Latest Changes from Upstream

Download all branches and commits from the official repository without merging:

```bash
git fetch upstream
```

This command downloads all the latest changes but doesn't modify your working directory.

### 4. Check Your Current Branch

Make sure you're on the branch you want to update (typically `main` or `master`):

```bash
git branch
```

If you need to switch branches:

```bash
git checkout main
```

### 5. Merge Upstream Changes into Your Fork

Choose one of the following approaches:

#### Option A: Merge (Preserves History)

This creates a merge commit and preserves the complete history:

```bash
git merge upstream/main
```

(Replace `main` with `master` if that's the upstream default branch)

#### Option B: Rebase (Cleaner History)

This replays your commits on top of the upstream changes, creating a linear history:

```bash
git rebase upstream/main
```

**Warning**: Only use rebase if you haven't pushed your commits yet, or if you're comfortable with force-pushing.

#### Option C: Create a Merge Commit Explicitly

This creates an explicit merge commit with a message:

```bash
git merge upstream/main --no-ff -m "Merge upstream rclone updates"
```

### 6. Resolve Any Conflicts (if they occur)

If there are merge conflicts:

1. Git will mark conflicted files in your working directory
2. Open the conflicted files and look for conflict markers:
   - `<<<<<<< HEAD` (your changes)
   - `=======` (separator)
   - `>>>>>>> upstream/main` (upstream changes)
3. Manually resolve the conflicts by editing the files
4. Stage the resolved files:
   ```bash
   git add <file>
   ```
5. Complete the merge:
   ```bash
   git commit
   ```
   Or if rebasing:
   ```bash
   git rebase --continue
   ```

### 7. Push to Your Fork

After successfully merging/rebase:

```bash
git push origin main
```

If you rebased and had already pushed before, you may need to force-push (use with caution):

```bash
git push origin main --force-with-lease
```

The `--force-with-lease` flag is safer than `--force` as it will fail if someone else has pushed changes you don't have.

## Quick Update Script

You can create a script to automate this process. Save this as `update-from-upstream.sh`:

```bash
#!/bin/bash
# Update fork from upstream

set -e  # Exit on error

echo "🔄 Fetching latest changes from upstream..."
git fetch upstream

# Get the default branch name (main or master)
UPSTREAM_BRANCH=$(git remote show upstream | grep "HEAD branch" | awk '{print $NF}')
CURRENT_BRANCH=$(git branch --show-current)

echo "📦 Current branch: $CURRENT_BRANCH"
echo "📦 Upstream branch: $UPSTREAM_BRANCH"

# Check for uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo "⚠️  Warning: You have uncommitted changes!"
    echo "   Please commit or stash them before updating."
    exit 1
fi

echo "🔀 Merging upstream/$UPSTREAM_BRANCH into $CURRENT_BRANCH..."
git merge upstream/$UPSTREAM_BRANCH

echo "✅ Successfully updated from upstream/$UPSTREAM_BRANCH"
echo ""
echo "📤 Next step: Push to your fork with:"
echo "   git push origin $CURRENT_BRANCH"
```

Make it executable:

```bash
chmod +x update-from-upstream.sh
```

## Important Notes

### Before Updating

1. **Commit or stash local changes**:
   ```bash
   git status  # Check for uncommitted changes
   git stash   # If needed, stash changes
   ```

2. **Create a backup branch** (optional but recommended):
   ```bash
   git branch backup-$(date +%Y%m%d)
   ```

### After Updating

1. **Test your changes** to ensure they still work with the updated codebase
2. **Review the changes** that were merged:
   ```bash
   git log upstream/main..HEAD  # See what's new
   ```

### Handling Uncommitted Changes

If you have uncommitted changes when you want to update:

1. **Option 1: Commit them first**
   ```bash
   git add .
   git commit -m "Your commit message"
   ```

2. **Option 2: Stash them**
   ```bash
   git stash
   # ... do the update ...
   git stash pop  # Restore your changes
   ```

3. **Option 3: Create a new branch**
   ```bash
   git checkout -b my-changes
   git add .
   git commit -m "Your commit message"
   git checkout main
   # ... do the update ...
   ```

## Troubleshooting

### Upstream remote doesn't exist

If you get an error that upstream doesn't exist:

```bash
git remote add upstream https://github.com/rclone/rclone.git
```

### Merge conflicts

If you encounter conflicts:

1. List conflicted files:
   ```bash
   git status
   ```

2. Resolve conflicts manually in each file

3. After resolving all conflicts:
   ```bash
   git add .
   git commit
   ```

### Accidentally merged wrong branch

If you merged the wrong branch:

```bash
git reset --hard HEAD~1  # Undo the last merge (destructive!)
```

Or use the reflog to find the commit before the merge:

```bash
git reflog
git reset --hard <commit-hash>
```

## Best Practices

1. **Regular updates**: Sync with upstream regularly to avoid large merge conflicts
2. **Test after merging**: Always test your code after merging upstream changes
3. **Use feature branches**: Keep your main branch clean and work on feature branches
4. **Review changes**: Use `git log` and `git diff` to review what changed
5. **Backup before force-push**: Always backup before using `--force` or `--force-with-lease`

## Official rclone Repository

- **URL**: `https://github.com/rclone/rclone`
- **Default branch**: `main` (as of recent updates, previously `master`)

## Related Commands

- `git remote show upstream` - Show detailed information about upstream
- `git log upstream/main..HEAD` - See commits in your fork not in upstream
- `git log HEAD..upstream/main` - See commits in upstream not in your fork
- `git diff upstream/main` - See differences between your branch and upstream
