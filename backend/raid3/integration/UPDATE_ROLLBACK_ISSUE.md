# Update Rollback Issue Analysis

**Date**: December 7, 2025  
**Status**: ⚠️ Known Issue - Update rollback not working properly  
**Priority**: High

---

## 🔍 Current Status

### Rollback Status by Operation

- ✅ **Put rollback**: Working correctly
- ✅ **Move rollback**: Working correctly (after fixes)
- ❌ **Update rollback**: Not working properly

---

## 🎯 Expected Behavior

According to the `rollback` option documentation and RAID 3 strict write policy:

1. **When `rollback=true` (default)**: Update operations should use move-to-temp pattern
   - Original particles are moved to temporary locations before updating
   - If update fails, original particles should be restored from temp locations
   - Ensures all-or-nothing guarantee - either all particles update, or none do

2. **When `rollback=false`**: Update operations should use in-place updates
   - Particles are updated directly without temp locations
   - No rollback on failure (partial updates allowed)

---

## ⚠️ Actual Behavior

### The Problem

Update rollback is implemented but **does not work correctly** when:
- Backends don't support server-side Move operations (e.g., S3/MinIO)
- Update operations fail partway through
- Backends become unavailable during update

### Known Issues

1. **Move-to-temp pattern fails with Copy+Delete backends**:
   - Original implementation relied on server-side `Move` operations
   - S3/MinIO backends only support `Copy` operations
   - Copy+Delete fallback was added but rollback restoration may fail

2. **Rollback restoration may not work**:
   - When update fails, particles should be moved back from temp locations
   - If backend doesn't support Move, Copy+Delete fallback may not work correctly
   - Original particles may not be restored, leaving files in degraded state

3. **Testing incomplete**:
   - No comprehensive tests for Update rollback scenarios
   - `update-fail` tests may not exist or may be skipped

---

## 📋 Implementation Details

### Current Implementation

The `Update()` function has two code paths:

1. **`updateInPlace()`**: Used when `rollback=false`
   - Updates particles directly without temp locations
   - No rollback mechanism

2. **`updateWithRollback()`**: Used when `rollback=true`
   - Uses `moveOrCopyParticleToTemp()` to move particles to temp locations
   - Applies updates to original locations
   - On failure, uses `rollbackUpdate()` to restore from temp locations

### Move-to-Temp Pattern

```go
// Pseudo-code of updateWithRollback flow
1. Move even particle to temp location (even_particle.tmp.even)
2. Move odd particle to temp location (odd_particle.tmp.odd)
3. Move parity particle to temp location (parity_particle.tmp.parity)
4. Apply update to original locations
5. If update succeeds:
   - Delete temp particles
6. If update fails:
   - Restore particles from temp locations (rollback)
   - Delete temp particles
```

### Potential Failure Points

1. **Step 1-3 (Move to temp)**:
   - Fails if backend doesn't support Move and Copy+Delete fails
   - Some particles may be at temp, others not

2. **Step 4 (Apply update)**:
   - Fails if any particle update fails
   - Original particles already moved to temp
   - New particles partially written

3. **Step 6 (Rollback)**:
   - Fails if Move back from temp doesn't work
   - Fails if Copy+Delete fallback doesn't work
   - Original particles remain at temp locations
   - Files in degraded state

---

## 🔍 Root Cause Analysis

### Why Update Rollback Doesn't Work

1. **Backend capability detection**:
   - May not correctly detect Copy vs Move support
   - `operations.CanServerSideMove()` might not be used consistently

2. **Error handling in rollback**:
   - `rollbackUpdate()` may not handle Copy+Delete fallback correctly
   - Errors during rollback may be logged but not propagated

3. **Temp file cleanup**:
   - Temp files may not be properly cleaned up on failure
   - Could leave orphaned temp files

4. **Incomplete implementation**:
   - Move-to-temp pattern was added but may not be fully tested
   - Edge cases not handled (backend unavailability, concurrent operations)

---

## 💡 Solutions

### Option 1: Fix Move-to-Temp Pattern

Improve `updateWithRollback()` to:
- Correctly handle Copy+Delete for backends without Move support
- Ensure rollback restoration works with Copy+Delete
- Add comprehensive error handling

### Option 2: Improve Rollback Restoration

Fix `rollbackUpdate()` to:
- Use `operations.CanServerSideMove()` consistently
- Implement Copy+Delete fallback for restoration
- Ensure temp files are always cleaned up

### Option 3: Add Comprehensive Testing

Create tests similar to `move-fail` scenarios:
- `update-fail-even`: Stop even backend, verify Update fails with rollback
- `update-fail-odd`: Stop odd backend, verify Update fails with rollback
- `update-fail-parity`: Stop parity backend, verify Update fails with rollback
- Verify original files are restored from temp locations
- Verify no temp files remain after rollback

### Option 4: Consider Alternative Approach

If move-to-temp pattern proves too complex:
- Use a different rollback strategy for Update
- Consider using versioned files instead of temp locations
- Or document that Update rollback has limitations with certain backends

---

## 📝 Testing Requirements

### Required Tests

1. **Update rollback with Move-supporting backends**:
   - Verify move-to-temp works
   - Verify rollback restoration works
   - Verify no temp files remain

2. **Update rollback with Copy-only backends (S3/MinIO)**:
   - Verify Copy+Delete works for move-to-temp
   - Verify Copy+Delete works for rollback restoration
   - Verify no temp files remain

3. **Update rollback with backend unavailability**:
   - Stop backend during move-to-temp phase
   - Stop backend during update phase
   - Stop backend during rollback phase
   - Verify proper cleanup in all cases

4. **Update rollback with partial failures**:
   - Some particles update successfully, others fail
   - Verify rollback restores all particles
   - Verify no degraded files created

---

## 🔗 Related Documentation

- `backend/raid3/raid3.go` - `Update()`, `updateWithRollback()`, `rollbackUpdate()` functions
- `backend/raid3/README.md` - Documents rollback for "Put, Update, Move" (needs update)

---

## 📋 Implementation History

### Initial Implementation

- Update rollback was implemented using move-to-temp pattern
- Similar to chunker backend approach
- Conditional on `rollback` option

### Known Fixes Applied

1. **Copy+Delete fallback for move-to-temp**:
   - Added `moveOrCopyParticleToTemp()` helper
   - Handles backends that don't support Move

2. **Copy+Delete fallback for Move operations**:
   - Added `moveOrCopyParticle()` helper
   - Used `operations.CanServerSideMove()` for capability detection

### Debugging Attempts

- Debugging was started in afternoon of December 7, 2025
- Attempted to fix rollback issues
- Changes were reverted to focus on Move rollback first
- Issue remains unresolved

---

## ✅ Next Steps

1. **Investigate current Update rollback implementation**:
   - Review `updateWithRollback()` and `rollbackUpdate()` code
   - Identify specific failure points
   - Test with different backend types (Move vs Copy-only)

2. **Create comprehensive tests**:
   - Add `update-fail` scenarios to `compare_raid3_with_single_errors.sh`
   - Test with MinIO (Copy-only backend)
   - Test with Move-supporting backends

3. **Fix identified issues**:
   - Ensure Copy+Delete fallback works correctly
   - Ensure rollback restoration works correctly
   - Ensure temp file cleanup always happens

4. **Update documentation**:
   - Update `README.md` to reflect current status
   - Document limitations if any remain
   - Document workarounds if needed

---

## 🎯 Success Criteria

Update rollback is considered fixed when:

- ✅ All `update-fail` tests pass
- ✅ Original files are restored on update failure
- ✅ No temp files remain after rollback
- ✅ Works with both Move-supporting and Copy-only backends
- ✅ Works when backends become unavailable during update
- ✅ All-or-nothing guarantee maintained (no degraded files)

---

**See also**: `backend/raid3/OPEN_QUESTIONS.md` - Q1: Update Rollback Not Working Properly

---

## 🔧 Fixed: InvalidBucketName Error (Path Join Bug + Dataset Name Length)

**Test**: `rollback-disabled-update-fail-parity`  
**Status**: ✅ **FIXED** (December 7, 2025)

### Problem (Fixed)

The test `rollback-disabled-update-fail-parity` was failing during dataset creation with:
```
ERROR: InvalidBucketName: The specified bucket is not valid
```

This error affected all three backends (even, odd, parity) and occurred **before** the backend was stopped.

### Root Cause

**Bug**: `path.Join()` was used to join remote paths with root paths, but `path.Join()` doesn't properly handle remote paths ending with `:`.

**Example**:
- `path.Join("minioeven:", "test")` produces `minioeven:/test` ❌ (wrong - leading slash)
- Should be: `minioeven:test` ✅

When the level3 backend constructed backend Fs objects with a root path (e.g., `miniolevel3:dataset_id`), it used `path.Join(opt.Even, root)` which created invalid paths like `minioeven:/dataset_id`. When S3 backend tried to parse these paths, it resulted in `InvalidBucketName` errors.

### Fix

Replaced all `path.Join(opt.Even/Odd/Parity, root)` calls with `fspath.JoinRootPath(opt.Even/Odd/Parity, root)`, which properly handles remote path construction.

**Files Fixed**:
- `backend/raid3/raid3.go` - Lines 547, 557, 567 (NewFs initial construction)
- `backend/raid3/raid3.go` - Lines 632, 641, 650 (adjusted root handling)

This matches the approach used by other virtual backends like `union` and `hasher`.

### Verification - Path Join Fix

**Before fix**:
```bash
$ path.Join("minioeven:", "test") → "minioeven:/test"  # Wrong - leading slash
```

**After fix**:
```bash
$ fspath.JoinRootPath("minioeven:", "test") → "minioeven:test"  # Correct
```

This fix ensures that backend paths are correctly constructed, preventing path parsing errors when level3 Fs objects are created with root paths.

When using MinIO/S3 backends without bucket names specified (e.g., `minioeven:`), the level3 root path is interpreted by S3 as a bucket name. S3 bucket names have a **63-character limit**. 

### Dataset Name Length Fix

**Additional Fix**: Changed dataset name prefix from `compare-` to `cmp-` to reduce length.

**Before**: `compare-rollback-disabled-update-fail-parity-20251207185729-1234` = 64 characters (over limit)  
**After**: `cmp-rollback-disabled-update-fail-parity-20251207185729-1234` = 61 characters (under 63-char limit) ✅

**Files Changed**:
- `backend/raid3/integration/compare_raid3_common.sh` - Changed prefix in `create_test_dataset()`
- `backend/raid3/integration/compare_raid3_with_single.sh` - Changed prefix in mkdir test

**Why this matters**: When using MinIO/S3 backends without bucket names specified (e.g., `minioeven:`), the level3 root path is interpreted by S3 as a bucket name. S3 bucket names have a **63-character limit**.

### Verification

Both fixes are working:
- ✅ Path join fix: Correct path construction using `fspath.JoinRootPath`
- ✅ Dataset name length: 61 characters (under 63-char limit)
- ✅ Test `rollback-disabled-update-fail-parity` now passes
