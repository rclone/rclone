# Bugfix: Auto-Cleanup Default Value

**Date**: November 5, 2025  
**Issue**: auto_cleanup was not defaulting to `true` when not specified in config  
**Status**: ✅ **FIXED**

---

## 🐛 The Bug

When `auto_cleanup` was not explicitly set in the rclone config file, it was defaulting to `false` instead of `true`, causing broken objects (1 particle) to appear in listings.

### Symptoms

```bash
# Setup
$ rclone config  # Created remote without specifying auto_cleanup
$ rclone ls miniolevel3:mybucket

# Expected: Broken objects hidden (auto_cleanup should default to true)
# Actual: Broken objects shown (auto_cleanup was defaulting to false)
```

### Root Cause

**Go's Zero Value Problem**:
- In Go, boolean fields default to `false` when not initialized
- When parsing config with `configstruct.Set()`, missing values stay at zero value
- Even though we specified `Default: true` in the option definition, this was not applied

**Before**:
```go
func NewFs(...) {
    opt := new(Options)
    err = configstruct.Set(m, opt)
    // If auto_cleanup not in config, opt.AutoCleanup = false (zero value)
}
```

---

## ✅ The Fix

Added explicit default value application in `NewFs()`:

```go
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (outFs fs.Fs, err error) {
    opt := new(Options)
    err = configstruct.Set(m, opt)
    if err != nil {
        return nil, err
    }

    // Apply default for auto_cleanup if not explicitly set
    // (Go's zero value is false, but our default should be true)
    if _, ok := m.Get("auto_cleanup"); !ok {
        opt.AutoCleanup = true
    }
    
    // ... rest of function
}
```

**After**:
- If `auto_cleanup` is not in config → defaults to `true` ✅
- If `auto_cleanup=true` in config → uses `true` ✅
- If `auto_cleanup=false` in config → uses `false` ✅

---

## 🧪 Testing

Added new test case: `TestAutoCleanupDefault`

```go
func TestAutoCleanupDefault(t *testing.T) {
    // Create level3 WITHOUT specifying auto_cleanup
    l3fs, err := level3.NewFs(ctx, "level3", "", configmap.Simple{
        "even":   evenDir,
        "odd":    oddDir,
        "parity": parityDir,
        // auto_cleanup NOT specified - should default to true
    })
    
    // Create broken object
    // Verify it's hidden from listings
}
```

**Test Results**:
```bash
$ go test -v -run="TestAutoCleanup"
=== RUN   TestAutoCleanupDefault
    level3_test.go:1711: ✅ Auto-cleanup defaults to true
--- PASS: TestAutoCleanupDefault (0.00s)
=== RUN   TestAutoCleanupEnabled
--- PASS: TestAutoCleanupEnabled (0.00s)
=== RUN   TestAutoCleanupDisabled
--- PASS: TestAutoCleanupDisabled (0.00s)
PASS
```

---

## 👤 User Action Required

### If You Experienced This Issue

Your existing level3 remote configurations **need to be updated** or **recreated**:

**Option 1: Add explicit setting** (recommended)
```bash
$ rclone config
# Select your level3 remote
# Edit advanced config
# Set auto_cleanup = true
```

**Option 2: Recreate remote**
```bash
$ rclone config delete miniolevel3
$ rclone config create miniolevel3 level3 \
    even remote1: \
    odd remote2: \
    parity remote3:
# auto_cleanup will now default to true
```

**Option 3: Manually edit config file**

Edit `~/.config/rclone/rclone.conf` (or your rclone config location):

```ini
[miniolevel3]
type = level3
even = remote1:
odd = remote2:
parity = remote3:
auto_cleanup = true  # Add this line
```

### After Fixing Config

Run cleanup to remove any broken objects that were accumulated:

```bash
$ rclone cleanup miniolevel3:mybucket
Scanning for broken objects...
Found 5 broken objects (total size: 3.0 KiB)
Cleaned up 5 broken objects (freed 3.0 KiB)
```

---

## 📊 Impact

**Who was affected**:
- Users who created level3 remotes without explicitly setting `auto_cleanup`
- Remotes created before this bugfix

**Severity**: Medium
- Broken objects appeared in listings (confusing)
- Purge operations showed error messages (but still worked)
- No data loss or corruption

**Fix availability**: Immediately available after update

---

## 🔍 How to Check If You're Affected

**Method 1: Check your config**
```bash
$ cat ~/.config/rclone/rclone.conf
[miniolevel3]
type = level3
even = ...
odd = ...
parity = ...
# If "auto_cleanup" line is missing, you're affected
```

**Method 2: Test behavior**
```bash
# Create a broken object manually (for testing)
# Then list - if you see it, auto_cleanup is off

$ rclone ls miniolevel3:mybucket
# If you see broken objects, auto_cleanup is disabled
```

---

## 📝 Related

- **Issue**: User reported broken objects appearing in listings
- **Analysis**: `docs/CONSISTENCY_PROPOSAL.md`
- **Implementation**: `docs/AUTO_CLEANUP_IMPLEMENTATION.md`
- **Complete**: `docs/AUTO_CLEANUP_COMPLETE.md`

---

## ✅ Verification

After applying the fix and updating config:

```bash
# Should show only valid objects
$ rclone ls miniolevel3:mybucket
     1024 valid1.txt
     2048 valid2.txt

# Cleanup any remaining broken objects
$ rclone cleanup miniolevel3:mybucket

# Verify clean
$ rclone config set miniolevel3 auto_cleanup false
$ rclone ls miniolevel3:mybucket
# Should show same list (no broken objects remain)
```

---

**Fix Status**: ✅ **Complete and Tested**

