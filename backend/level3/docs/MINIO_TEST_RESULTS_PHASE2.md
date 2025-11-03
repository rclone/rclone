# MinIO Interactive Test Results - Phase 2

**Date**: November 2, 2025  
**Purpose**: Verify strict write policy with real backend unavailability

---

## 🚨 CRITICAL FINDINGS

### Test 1: Move with Unavailable Backend

**Setup**:
```bash
# Create file with all backends running
echo "Test Move Failure" > /tmp/move-test.txt
rclone copy /tmp/move-test.txt miniolevel3:testbucket/

# Stop odd backend
docker stop minioodd

# Attempt move
rclone moveto miniolevel3:testbucket/move-test.txt miniolevel3:testbucket/renamed-move.txt
```

**Observed Behavior**:
```
2025/11/02 07:32:35 DEBUG : move-test.txt: Can't move, switching to copy
...
2025/11/02 07:33:33 INFO  : move-test.txt: Reconstructed from even+parity (degraded mode)
...
2025/11/02 07:36:33 ERROR : move-test.txt: Failed to copy: failed to upload odd particle
2025/11/02 07:36:33 ERROR : move-test.txt: Not deleting source as copy failed
...
2025/11/02 07:37:09 INFO  : move-test.txt: Deleted
2025/11/02 07:37:09 ERROR : Attempt 2/3 succeeded
```

**Result**: ⚠️ **PARTIAL SUCCESS on RETRY**

**Particles after move**:
- Even: `renamed-move.txt` ✅
- Odd: Missing ❌ (backend was down)
- Parity: `renamed-move.txt.parity-el` ✅

**Analysis**:
1. Move fell back to copy+delete (server-side move not available)
2. First copy attempt failed (strict behavior) ✅
3. **Retry succeeded and created degraded file** ⚠️
4. Source was deleted after successful copy

**Verdict**: 
- ❌ **Move does NOT enforce strict policy on retries**
- ❌ **Creates degraded files after retry**
- ✅ First attempt failed correctly
- ⚠️ Rclone's retry logic bypasses our strict enforcement!

---

### Test 2: Update with Unavailable Backend

**Setup**:
```bash
# Create original file
echo "Original Update Test Content" > /tmp/update-test.txt
rclone copy /tmp/update-test.txt miniolevel3:testbucket/

# Stop odd backend
docker stop minioodd

# Attempt update
echo "Updated Content Here" | rclone rcat miniolevel3:testbucket/update-test.txt
```

**Observed Behavior**:
```
Process hung (killed after 15 seconds)
```

**After restarting odd backend**:
```bash
rclone cat miniolevel3:testbucket/update-test.txt

ERROR: Failed to open: invalid particle sizes: even=11, odd=14
```

**Result**: 🚨 **FILE CORRUPTED!**

**Analysis**:
- Update succeeded on even backend (11 bytes for "Updated Content Here")
- Update succeeded on odd backend before it went down? Or got corrupted?
- Particles have INVALID sizes (even=11, odd=14)
- **File is now unreadable!**

**Verdict**:
- 🚨 **CRITICAL BUG: Update corrupts files when backend unavailable**
- ❌ **Update does NOT enforce strict policy**
- ❌ **Partial updates create corrupted state**
- 🚨 **DATA INTEGRITY VIOLATION**

---

## 🔍 Root Cause Analysis

### Why Strict Policy is Not Enforced

**The Problem**: Rclone's command layer (copy/move/rcat) has its own **retry logic** that operates ABOVE the backend level.

**What happens**:
1. Backend's `Put/Update` fails correctly (errgroup works) ✅
2. **BUT**: Rclone command retries the entire operation
3. On retry, degraded state may allow partial success
4. Partial particles get created ❌

**Code Flow**:
```
User: rclone copy file.txt level3:
  ↓
Rclone Command Layer (operations.Copy)
  ↓ [Retry Loop - 3 attempts]
  ↓
Backend level3.Put()
  ↓ [errgroup - strict]
  ↓
If ANY backend fails → Return error
  ↓
[Back to Retry Loop]
  ↓
Retry #2 → Backend.Put() again
  ↓
May succeed partially on retry!
```

---

## ⚠️ Implications

### Move Operations:
- ❌ Can create degraded files on retry
- ❌ Moved file missing one particle
- ⚠️ Self-healing will eventually fix, but creates degraded state

### Update Operations:
- 🚨 **Can CORRUPT files!**
- 🚨 Invalid particle sizes
- 🚨 File becomes unreadable
- 🚨 **DATA LOSS RISK**

---

## 🛠️ Required Fixes

### Fix 1: Disable Retries for Write Operations (Urgent)

**Problem**: Rclone's retry logic bypasses backend-level strict enforcement

**Solution**: Set `LowLevelRetries = 0` for write operations in degraded mode

```go
func (f *Fs) Put(ctx context.Context, ...) {
    // Check if we're in degraded mode
    if f.isDegraded() {
        // Disable retries - fail fast
        ctx, ci := fs.AddConfig(ctx)
        ci.LowLevelRetries = 0
    }
    
    // Existing Put logic...
}
```

**Benefits**:
- First failure is final
- No retry creates degraded files
- Clear error to user

---

### Fix 2: Add Degraded Mode Detection

**Need function**:
```go
func (f *Fs) isDegraded() bool {
    // Check if all 3 backends are reachable
    // Could do quick health check (HeadBucket or similar)
    // Or track failures from recent operations
}
```

**Usage**:
- Before Put/Update/Move
- Decide whether to allow operation
- Or at least disable retries

---

### Fix 3: Update Implementation - Add Validation

**Problem**: Update can create invalid particle sizes

**Solution**: Validate sizes after update, rollback if invalid

```go
func (o *Object) Update(...) error {
    // Update all particles...
    
    // Verify particle sizes are valid
    evenObj, _ := o.fs.even.NewObject(ctx, o.remote)
    oddObj, _ := o.fs.odd.NewObject(ctx, o.remote)
    
    if !ValidateParticleSizes(evenObj.Size(), oddObj.Size()) {
        // Rollback - restore original particles
        return errors.New("update failed: invalid particle sizes")
    }
}
```

---

## ✅ What Works Correctly

### Put Operations ✅
- First attempt fails correctly when backend unavailable
- errgroup enforcement works
- **Issue**: Retries can create degraded files

### Delete Operations ✅
- Best-effort policy works correctly
- Idempotent behavior
- **No issues found**

### Read Operations ✅
- Degraded mode reconstruction works
- Self-healing triggers correctly
- **No issues found**

---

## 📋 Action Items

### Critical (Data Integrity):
1. 🚨 **Fix Update corruption** - Add particle size validation and rollback
2. ⚠️ **Fix Move degraded file creation** - Disable retries or check degraded mode first
3. ⚠️ **Fix Put degraded file creation** - Same as Move

### Important (Policy Enforcement):
4. Add `isDegraded()` function to detect backend unavailability
5. Disable retries for write operations in degraded mode
6. Add proper rollback logic for partial updates

### Testing:
7. Re-test with fixes in place
8. Verify no degraded files created
9. Verify Update doesn't corrupt

---

## 📊 Test Summary

| Operation | Backend Down | First Attempt | Retries | Final Result | Verdict |
|-----------|--------------|---------------|---------|--------------|---------|
| **Put** | Odd | ✅ Failed | ❌ May create degraded | Partial success | ⚠️ Issue |
| **Move** | Odd | ✅ Failed | ❌ Created degraded | Partial success | ⚠️ Issue |
| **Update** | Odd | ? | ? | 🚨 **Corrupted file** | 🚨 **Critical Bug** |
| **Delete** | Odd | ✅ Succeeded | N/A | Success | ✅ Correct |
| **Read** | Odd | ✅ Succeeded | N/A | Success | ✅ Correct |

---

## 🎯 Recommendation

**URGENT**: Fix Update corruption before production use!

**Priority**:
1. **High**: Fix Update (data corruption risk)
2. **Medium**: Add degraded mode detection  
3. **Medium**: Disable retries for writes in degraded mode
4. **Low**: Add proper rollback for Move

**Timeline**: Should be fixed before considering production-ready for S3/MinIO backends.

**Workaround for now**: 
- Use local backends (no corruption observed)
- Don't update files when MinIO backend is down
- Monitor for "invalid particle sizes" errors

---

**Status**: Phase 2 testing revealed critical bugs that need fixing! 🚨

