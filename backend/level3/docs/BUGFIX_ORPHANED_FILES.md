# Bugfix: Orphaned Files in Parity Remote Not Cleaned Up

**Date**: November 5, 2025  
**Issue**: Files in parity remote without proper suffixes were not detected or cleaned up  
**Status**: ✅ **FIXED**

---

## 🐛 The Bug

The `rclone cleanup` command was not removing orphaned files that existed in the parity remote without proper `.parity-el` or `.parity-ol` suffixes. This affected:
- Manually created files in parity remote
- Files that were partially deleted (remaining in only 1 remote)
- Any non-level3-created files in the parity remote

### User's Scenario

```bash
# Setup
# User had 'hallo_fr.txt' manually created in all 3 remotes
# Then deleted from 2 remotes, leaving only in parity remote

$ rclone cleanup miniolevel3:mybucket
# Expected: Should find and delete the orphaned file
# Actual: Did nothing - file remained

$ rclone ls miniolevel3:mybucket/hallo_fr.txt  
# File still exists in parity remote
```

### Root Cause

The `scanParticles` function only tracked files in the parity remote that had proper parity suffixes:

**Before (buggy code)**:
```go
// Process parity particles
for _, entry := range entriesParity {
    remote := entry.Remote()
    baseRemote, isParity, _ := StripParitySuffix(remote)
    if isParity {
        // Only processes files WITH .parity-el or .parity-ol suffix
        objectMap[baseRemote].parityExists = true
    }
    // Files without suffix were COMPLETELY IGNORED!
}
```

**Problem**: If a file exists in the parity remote without a suffix (like `hallo_fr.txt`), it would be:
1. Not added to `objectMap`
2. Not detected as a broken object
3. Not cleaned up by `rclone cleanup`

---

## ✅ The Fix

Updated `scanParticles` to also track files in parity remote without suffixes:

```go
// Process parity particles
for _, entry := range entriesParity {
    remote := entry.Remote()
    baseRemote, isParity, _ := StripParitySuffix(remote)
    if isParity {
        // Proper parity file with suffix
        objectMap[baseRemote].parityExists = true
    } else {
        // File in parity remote without suffix (orphaned/manually created)
        // Still track it as it might be a broken object
        objectMap[remote].parityExists = true  // NEW!
    }
}
```

Also updated `removeBrokenObject` and `getBrokenObjectSize` to handle files without suffixes:

```go
// Try removing from parity remote
// Try with suffixes
if err := tryRemoveWithSuffix(parityOL); err == nil { return }
if err := tryRemoveWithSuffix(parityEL); err == nil { return }
// Also try without suffix (for orphaned files) - NEW!
if err := tryRemove(p.remote); err == nil { return }
```

---

## 🧪 Testing

Added new test case: `TestCleanUpOrphanedFiles`

```go
func TestCleanUpOrphanedFiles(t *testing.T) {
    // Create orphaned files in all three backends WITHOUT proper suffixes
    orphan1 := createOrphanInEven("orphan1.txt")
    orphan2 := createOrphanInOdd("orphan2.txt")
    orphan3 := createOrphanInParity("orphan3.txt")  // ← The critical test!
    
    // Run cleanup
    l3fs.CleanUp(ctx)
    
    // Verify all orphaned files are deleted
    assertDeleted(orphan1)
    assertDeleted(orphan2)
    assertDeleted(orphan3)  // ← This was failing before the fix!
}
```

**Test Results**:
```bash
$ go test -v -run="TestCleanUpOrphanedFiles"
=== RUN   TestCleanUpOrphanedFiles
    level3_test.go:2109: ✅ CleanUp successfully removed orphaned files 
                          including those in parity without suffix
--- PASS: TestCleanUpOrphanedFiles (0.00s)
PASS
```

All 7 auto-cleanup tests passing ✅

---

## 👤 What You Need to Do

Now that the bug is fixed, **you can actually clean up your orphaned files**:

### Step 1: Update Config (if not done already)

Ensure `auto_cleanup=true` in your config:

```ini
[miniolevel3]
type = level3
even = minio1:
odd = minio2:
parity = minio3:
auto_cleanup = true
```

### Step 2: Run Cleanup Command

```bash
$ rclone cleanup miniolevel3:mybucket
Scanning for broken objects...
Found 1 broken object: hallo_fr.txt (1 particle)
Cleaning up broken object: hallo_fr.txt (1 particle)
Cleaned up 1 broken object (freed X bytes)
```

### Step 3: Verify File is Gone

Check the remote where it existed:

```bash
# If it was in minio3 (parity remote):
$ rclone ls minio3:mybucket | grep hallo_fr.txt
# Should return nothing - file is gone
```

Or check via level3:

```bash
$ rclone ls miniolevel3:mybucket
# hallo_fr.txt should not appear (hidden by auto_cleanup)

$ rclone config set miniolevel3 auto_cleanup false
$ rclone ls miniolevel3:mybucket
# hallo_fr.txt should STILL not appear (actually deleted now)
```

---

## 📊 What This Fixes

**Scenarios now handled**:
- ✅ Files manually created in any remote without proper level3 structure
- ✅ Files remaining in parity remote without suffix after partial deletion
- ✅ Files in even or odd remotes (already worked, now more robust)
- ✅ Mixed scenarios (some with suffixes, some without)

**What it doesn't fix**:
- ❌ Valid level3 files (3 particles) - these are kept (correct behavior)
- ❌ Valid degraded files (2 particles) - these are kept (correct behavior)
- ❌ Directories - these are not files

---

## 🔍 Related Issues

This bug was related to the first bug ([BUGFIX_AUTO_CLEANUP_DEFAULT.md](BUGFIX_AUTO_CLEANUP_DEFAULT.md)):

**Bug 1**: Default value not applied
- **Symptom**: Broken objects appeared in listings
- **Fix**: Apply default `auto_cleanup=true` in NewFs()

**Bug 2**: Orphaned files not detected (THIS BUG)
- **Symptom**: `rclone cleanup` didn't remove orphaned files in parity remote
- **Fix**: Track files in parity remote without suffixes

Both bugs needed to be fixed for cleanup to work correctly!

---

## 📝 Technical Details

### Why Parity Files Have Suffixes

In level3's RAID 3 design:
- Even particles: stored as-is (e.g., `file.txt`)
- Odd particles: stored as-is (e.g., `file.txt`)  
- Parity particles: stored with suffix (e.g., `file.txt.parity-ol` or `file.txt.parity-el`)

The suffix indicates which "length" the parity is for:
- `.parity-ol` = odd-length (odd bytes > even bytes)
- `.parity-el` = even-length (even bytes ≥ odd bytes)

### Why This Bug Existed

The original code **assumed** all files were properly created by level3's Put operation. It didn't account for:
1. Manual file creation by users
2. Partial deletions leaving orphaned particles
3. Migration from other systems
4. Testing/debugging scenarios

The fix makes cleanup more robust by handling **any** files in the remotes, not just properly-structured level3 files.

---

## ✅ Verification

After the fix, you should be able to:

```bash
# Clean up any orphaned files
$ rclone cleanup miniolevel3:mybucket
Cleaned up X broken objects

# Verify cleanup worked
$ rclone config set miniolevel3 auto_cleanup false
$ rclone ls miniolevel3:mybucket
# Should only show valid objects (2+ particles)

# Check physical remotes
$ rclone ls minio1:mybucket  # even
$ rclone ls minio2:mybucket  # odd  
$ rclone ls minio3:mybucket  # parity
# Should be in sync - no orphaned files
```

---

## 📊 Summary

**Files Changed**:
- `level3.go`: scanParticles() - track files without suffixes
- `level3.go`: removeBrokenObject() - remove files without suffixes
- `level3.go`: getBrokenObjectSize() - get size for files without suffixes
- `level3_test.go`: TestCleanUpOrphanedFiles() - test case for this scenario

**Impact**: 
- Critical fix for cleanup functionality
- Affects users with manually created files or partial deletions
- No impact on properly-structured level3 files

**Status**: ✅ **Fixed, tested, and ready for use**

---

**Your manual testing discovered two critical bugs - thank you!** 🙏

