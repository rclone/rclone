# Move Failure Test Analysis

**Date**: December 4, 2025  
**Test**: `compare_level3_with_single_errors.sh test move-fail-even`

---

## 🔍 What the Test Detects

The test correctly detects that **partial moves occur** when a backend becomes unavailable during a move operation.

### Test Results

```
✓ Move command returns non-zero exit code (move failed)
✗ New file exists at destination (partial move occurred)
  - Odd particle moved to new location
  - Parity particle moved to new location
  - Even particle failed (backend unavailable)
```

---

## 🎯 Expected Behavior

According to RAID 3 strict write policy (documented in `ERROR_HANDLING_POLICY.md`):

1. **Pre-flight Check**: Move should fail at `checkAllBackendsAvailable()` if any backend is unavailable
2. **No Partial Moves**: If move starts, all particles must move atomically
3. **Rollback**: If move fails partway, completed moves should be rolled back

---

## ⚠️ Actual Behavior

The test reveals:

1. **Move command fails** (non-zero exit) ✓
2. **But partial move occurs** (some particles moved) ✗
3. **No rollback** (completed moves not undone) ✗

### Error Sequence

```
1. Backend stopped (even backend unavailable)
2. Move command attempted
3. Error: HEAD operation fails (checking destination)
   "dial tcp 127.0.0.1:9001: connect: connection refused"
4. Move command returns error (exit code non-zero)
5. BUT: Odd and parity particles already at new location!
```

---

## 📋 Known Limitations

From `ERROR_HANDLING_POLICY.md` (lines 186-219):

### Problem: No Rollback for Move

```
Move even:   ✅ Completed before error
Move odd:    ❌ Failed
Move parity: 🔄 Cancelled by context
```

**Result**: Even particle at new location, odd/parity at old location!

### Current Implementation

- Uses `errgroup` for parallel moves
- Context cancellation stops pending moves
- **BUT**: Already-completed moves aren't undone

---

## 🤔 Root Cause Analysis

### Why Does Partial Move Occur?

1. **Pre-flight check timing**: The `checkAllBackendsAvailable()` check happens, but:
   - Backend might be reachable during check
   - Backend fails between check and move
   - Race condition between check and move operations

2. **Rclone operations layer**: 
   - Does HEAD check before calling backend Move()
   - HEAD check fails (backend unavailable)
   - But Move() might have already been attempted
   - Or particles moved before HEAD check completed

3. **No rollback mechanism**:
   - Move operations don't track which particles moved successfully
   - No mechanism to undo completed moves
   - Error is returned, but state is inconsistent

---

## 💡 Solutions

### Option 1: Improve Pre-flight Check Timing

Make the pre-flight check more robust:
- Check backends immediately before move operations
- Reduce time window between check and move
- Add retry logic with backoff

### Option 2: Implement Rollback Mechanism

Track successful moves and rollback on failure:
```go
func (f *Fs) Move(...) {
    var movedEven, movedOdd, movedParity bool
    
    // Track which moves succeed
    // If any fail, rollback completed moves
}
```

### Option 3: Make Move Atomic

Use two-phase commit:
1. **Phase 1**: Prepare move on all backends (lock/verify)
2. **Phase 2**: Commit move on all backends (actual move)
3. If any backend fails in Phase 1, abort before Phase 2

---

## 📝 Test Status

**Current Status**: Test correctly detects partial move scenario

**Recommendation**: 
- ✅ Keep test as-is (it's working correctly)
- ✅ Document as known limitation
- ✅ Use test to validate when rollback is implemented
- ⚠️ Consider adding rollback mechanism to Move implementation

---

## 🔗 Related Documentation

- `backend/level3/docs/ERROR_HANDLING_POLICY.md` - Official error handling policy
- `backend/level3/docs/STRICT_WRITE_FIX.md` - Strict write policy implementation
- `backend/level3/level3.go` - Move implementation (lines 2122-2212)

---

## ✅ Conclusion

The test is **working correctly** - it successfully detects that partial moves occur when backends become unavailable. This is a known limitation that should be addressed by implementing rollback in the Move operation.

**Next Steps**:
1. Acknowledge test result as expected (detects known limitation)
2. Document need for rollback mechanism
3. Consider implementing rollback in future version
4. Update test to verify rollback once implemented


