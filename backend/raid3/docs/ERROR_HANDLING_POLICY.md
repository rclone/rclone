# Error Handling Policy - RAID 3 Compliance

**Date**: November 2, 2025  
**Decision**: Option A (Strict) - Hardware RAID 3 Compliant

---

## 🎯 Official Policy

The level3 backend follows **hardware RAID 3 behavior** for error handling:

### Read Operations: **Best Effort** (Degraded Mode Supported)
- ✅ Work with ANY 2 of 3 backends available
- ✅ Automatic reconstruction from parity
- ✅ Self-healing restores missing particles
- ✅ Performance: 6-7 seconds (S3 aggressive mode)

### Write Operations: **Atomic** (All Backends Required)
- ❌ **Require ALL 3 backends available**
- ❌ **Fail fast** if any backend unavailable
- ❌ Do NOT create degraded files
- ✅ Ensures consistency

### Delete Operations: **Best Effort** (Idempotent)
- ✅ Succeed if any backend reachable
- ✅ Ignore "not found" errors
- ✅ Idempotent (can delete multiple times)

---

## 📚 Rationale

### Why Strict Writes?

**Hardware RAID 3 Compliance**:
- Industry standard behavior
- All hardware RAID 3 controllers block writes in degraded mode
- Linux MD RAID default behavior
- Proven approach over 30+ years

**Data Safety**:
- Prevents creating partially-written files
- No inconsistent state from the start
- Every file is either fully replicated or not created at all

**Simplicity**:
- No complex rollback logic needed
- No partial state to manage
- Clear error messages

**Performance**:
- Avoids performance degradation from constant reconstruction
- New files don't require parity reconstruction
- Self-healing only for pre-existing degraded files

---

## 🔄 Behavior by Operation

### 1. Put (Create File)

**Normal Mode** (all 3 backends available):
```go
Upload to even:   ✅ Success
Upload to odd:    ✅ Success
Upload to parity: ✅ Success
Result: File created ✅
```

**Degraded Mode** (one backend down):
```go
Upload to even:   ✅ Success
Upload to odd:    ❌ Backend unavailable
Upload to parity: ✅ Success
Result: ERROR - Put failed, even/parity uploads automatically rolled back by errgroup
```

**User sees**:
```
ERROR: Failed to upload file.txt: odd backend unavailable
```

---

### 2. Move (Rename File)

**Normal Mode** (all 3 backends available):
```go
Move even particle:   ✅ Success
Move odd particle:    ✅ Success
Move parity particle: ✅ Success
Result: File renamed ✅
```

**Degraded Mode** (one backend down):
```go
Move even particle:   ✅ Success
Move odd particle:    ❌ Backend unavailable
Move parity particle: ✅ Success
Result: ERROR - Move failed, need to rollback even/parity moves
```

**Rollback** (to be implemented):
- Delete/restore even particle at new location
- Delete/restore parity particle at new location
- Return error to user

**User sees**:
```
ERROR: Failed to move file.txt: odd backend unavailable
```

---

### 3. Remove (Delete File)

**Normal Mode** (all 3 backends available):
```go
Delete even particle:   ✅ Success
Delete odd particle:    ✅ Success
Delete parity particle: ✅ Success
Result: File deleted ✅
```

**Degraded Mode** (one backend down):
```go
Delete even particle:   ✅ Success
Delete odd particle:    ❌ Backend unavailable (ignored)
Delete parity particle: ✅ Success
Result: File deleted ✅ (odd particle left orphaned, but backend is down anyway)
```

**Partial Particle Missing**:
```go
Delete even particle:   ✅ Success
Delete odd particle:    ❌ Not found (ignored - idempotent)
Delete parity particle: ✅ Success
Result: File deleted ✅
```

**Rationale for Best Effort Delete**:
- Missing particle = already deleted (same end state)
- Idempotent delete is user-friendly
- Can't make state worse by deleting

---

### 4. Update (Modify File)

**Normal Mode** (all 3 backends available):
```go
Update even particle:   ✅ Success
Update odd particle:    ✅ Success
Update parity particle: ✅ Success
Result: File updated ✅
```

**Degraded Mode** (one backend down):
```go
Update even particle:   ✅ Success
Update odd particle:    ❌ Backend unavailable
Update parity particle: ✅ Success
Result: ERROR - Update failed
```

**Behavior**: Same as Put (strict)

---

## 🛡️ Error Handling Details

### Put/Update/Move Failure Handling

**Current Implementation** (using `errgroup`):
- Uploads/moves happen in parallel
- If ANY goroutine returns error, context is cancelled
- Other operations are automatically cancelled
- errgroup.Wait() returns first error

**Result**: Automatic rollback via context cancellation ✅

**Limitation**: Already-completed operations aren't undone!

---

### Rollback Strategy for Move

**Problem**: If Move fails partway through:
```
Move even:   ✅ Completed before error
Move odd:    ❌ Failed
Move parity: 🔄 Cancelled by context
```

**Result**: Even particle at new location, odd/parity at old location!

**Solution Needed**:
```go
func (f *Fs) Move(...) {
    // Track successful moves
    var movedEven, movedOdd, movedParity bool
    
    // Attempt moves in parallel
    // ... errgroup logic ...
    
    // If any failed, rollback
    if err != nil {
        if movedEven {
            // Delete even from new location or move back
        }
        if movedParity {
            // Delete parity from new location or move back
        }
        return err
    }
}
```

**Complexity**: Medium (need to track what succeeded)

---

## 📊 Comparison with Current Implementation

| Operation | Current Behavior | Option A Behavior | Change Needed? |
|-----------|------------------|-------------------|----------------|
| **NewObject** | Works with 2 of 3 | Same | ✅ Already correct |
| **Open** | Works with 2 of 3 | Same | ✅ Already correct |
| **Put** | Works with 2 of 3? | Require all 3 | ⚠️ Verify/enforce |
| **Update** | Works with 2 of 3? | Require all 3 | ⚠️ Verify/enforce |
| **Move** | Works with 2 of 3? | Require all 3 + rollback | ⚠️ Add rollback |
| **Remove** | Ignores missing | Same | ✅ Already correct |
| **Self-healing** | Background upload | Same | ✅ Already correct |

---

## ✅ Implementation Checklist

### 1. Verify Current Behavior ✅

**Put**: Check if it already fails when backend unavailable
**Update**: Check if it already fails when backend unavailable
**Move**: Check if it has rollback logic

### 2. Add Explicit Checks (If Needed)

```go
func (f *Fs) Put(...) {
    // Before upload, verify all backends available?
    // Or let errgroup handle it naturally?
}
```

### 3. Add Rollback to Move

```go
func (f *Fs) Move(...) {
    // Track successful moves
    // Rollback on error
}
```

### 4. Update Documentation

- ✅ README.md - Add error handling section
- ✅ RAID3.md - Document RAID 3 compliance
- ✅ Tests - Update comments to mention strict writes

### 5. Add Phase 2 Tests

- Test Put fails when backend unavailable
- Test Move fails when backend unavailable  
- Test Move rollback works
- Test Delete succeeds with missing particles

---

## 🎉 Decision Summary

**DECIDED**: Option A (Strict) - Hardware RAID 3 Compliant

**Policy**:
- ✅ Reads: Best effort (degraded mode supported)
- ❌ Writes: Atomic (all 3 backends required)
- ✅ Deletes: Best effort (idempotent)

**Next Steps**:
1. ✅ Document this decision
2. ⏳ Update README with error handling policy
3. ⏳ Verify current Put/Update/Move behavior
4. ⏳ Add rollback to Move if needed
5. ⏳ Implement Phase 2 tests

---

**This decision makes level3 a true RAID 3 implementation!** 🎯

