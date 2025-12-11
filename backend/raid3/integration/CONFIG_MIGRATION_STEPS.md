# Detailed Rclone Config Migration Steps

## Overview

After renaming the backend from `level3` to `raid3`, you need to update your `rclone.conf` file. The key change is updating the `type` parameter from `level3` to `raid3`.

---

## Step-by-Step Migration

### Step 1: Locate Your Config File

Your rclone config file is typically located at:
- **Linux/Mac**: `~/.config/rclone/rclone.conf`
- **Windows**: `%APPDATA%\rclone\rclone.conf`

### Step 2: Backup Your Config (Recommended)

```bash
cp ~/.config/rclone/rclone.conf ~/.config/rclone/rclone.conf.backup
```

### Step 3: Edit the Config File

Open the config file in your preferred editor:
```bash
# Using your default editor
rclone config file

# Or directly
nano ~/.config/rclone/rclone.conf
# or
vi ~/.config/rclone/rclone.conf
```

### Step 4: Find All `level3` Remote Sections

Look for sections that look like this:

```ini
[locallevel3]
type = level3
even = localeven:/some/path
odd = localodd:/some/path
parity = localparity:/some/path
auto_cleanup = true
auto_heal = true
rollback = true

[miniolevel3]
type = level3
even = minioeven:bucket-name
odd = minioodd:bucket-name
parity = minioparity:bucket-name
auto_cleanup = true
auto_heal = true
rollback = true
```

### Step 5: Change the Backend Type

For **each** remote section that has `type = level3`, change it to `type = raid3`:

**Before:**
```ini
[locallevel3]
type = level3
...
```

**After:**
```ini
[locallevel3]
type = raid3
...
```

### Step 6: (Optional) Rename Remote Sections

If you want to use the new naming convention, you can also rename the section headers:

**Before:**
```ini
[locallevel3]
type = raid3
...
```

**After:**
```ini
[localraid3]
type = raid3
...
```

**Important**: If you rename the section, you'll need to:
1. Update any scripts or automation that reference the old name
2. If using a custom remote name, set `RAID3_REMOTE` environment variable to match your config

---

## Complete Example

### Before Migration:

```ini
[locallevel3]
type = level3
even = localeven:/home/user/raid3/even
odd = localodd:/home/user/raid3/odd
parity = localparity:/home/user/raid3/parity
auto_cleanup = true
auto_heal = true
rollback = true

[miniolevel3]
type = level3
even = minioeven:mybucket
odd = minioodd:mybucket
parity = minioparity:mybucket
auto_cleanup = true
auto_heal = true
rollback = true
```

### After Migration (Option 1 - Minimal Changes):

```ini
[locallevel3]
type = raid3              ← ONLY THIS LINE CHANGED
even = localeven:/home/user/raid3/even
odd = localodd:/home/user/raid3/odd
parity = localparity:/home/user/raid3/parity
auto_cleanup = true
auto_heal = true
rollback = true

[miniolevel3]
type = raid3              ← ONLY THIS LINE CHANGED
even = minioeven:mybucket
odd = minioodd:mybucket
parity = minioparity:mybucket
auto_cleanup = true
auto_heal = true
rollback = true
```

### After Migration (Option 2 - Full Rename):

```ini
[localraid3]              ← SECTION NAME CHANGED
type = raid3              ← TYPE CHANGED
even = localeven:/home/user/raid3/even
odd = localodd:/home/user/raid3/odd
parity = localparity:/home/user/raid3/parity
auto_cleanup = true
auto_heal = true
rollback = true

[minioraid3]              ← SECTION NAME CHANGED
type = raid3              ← TYPE CHANGED
even = minioeven:mybucket
odd = minioodd:mybucket
parity = minioparity:mybucket
auto_cleanup = true
auto_heal = true
rollback = true
```

---

## What to Change

### ✅ MUST Change:
- **`type = level3`** → **`type = raid3`** (for all level3 remotes)

### ⚠️ Optional (but recommended):
- **`[locallevel3]`** → **`[localraid3]`** (section name)
- **`[miniolevel3]`** → **`[minioraid3]`** (section name)

### ❌ DON'T Change:
- `even`, `odd`, `parity` remote names - these can stay the same
- `auto_cleanup`, `auto_heal`, `rollback` options - these work the same
- File paths or bucket names
- Any other configuration options

---

## Verification Steps

After making changes, verify your configuration:

### 1. Test Config Syntax
```bash
rclone config show locallevel3    # or localraid3 if you renamed it
```

Should show:
```
[locallevel3]
type = raid3
...
```

### 2. Test Remote Access
```bash
rclone lsd locallevel3:
```

Should list directories (or show empty if no data).

### 3. Test Backend Commands
```bash
rclone backend status locallevel3:
```

Should show backend health status.

---

## Troubleshooting

### Error: "didn't find backend called 'level3'"
→ You forgot to change `type = level3` to `type = raid3`

### Error: "didn't find section in config file"
→ The remote name doesn't match. Check:
- Section name in config matches the expected name (`localraid3` or `minioraid3` by default)
- Or set `RAID3_REMOTE` environment variable to match your config

### Error: "No remotes found"
→ Check that:
- Section header matches (case-sensitive)
- No typos in the section name
- Config file is in the correct location

---

## After Migration

Once you've updated your config:

1. **Test your scripts:**
   ```bash
   ./compare_raid3_with_single.sh test --storage-type=local -v
   ./compare_raid3_with_single.sh test --storage-type=minio -v
   ```

2. **If your config uses custom remote names:**
   - Set `RAID3_REMOTE` environment variable to match your config
   - Or create `compare_raid3_env.local.sh` with your custom remote names

3. **Update any automation:**
   - Scripts or automation that reference the remote name
   - Documentation or notes

---

## Quick Reference

| Item | Before | After |
|------|--------|-------|
| Section name | `[locallevel3]` | `[locallevel3]` or `[localraid3]` (your choice) |
| Backend type | `type = level3` | `type = raid3` |
| Other options | Unchanged | Unchanged |

**Minimum required change**: `type = level3` → `type = raid3`

**Note**: Section names can be anything you prefer. If you use a custom name, set `RAID3_REMOTE` environment variable to match your config.



