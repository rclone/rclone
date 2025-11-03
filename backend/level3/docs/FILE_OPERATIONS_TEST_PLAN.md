# File Operations Test Plan

**Date**: November 2, 2025  
**Purpose**: Comprehensive test coverage for create, rename, delete, and move operations

---

## 📋 Operations to Test

### 1. Create File (Put)
- ✅ **Already tested** in `TestStandard`
- Verifies: All 3 particles created, data split correctly

### 2. Rename File (Move)
- ❌ **Not yet tested** - needs implementation
- Verifies: All 3 particles renamed, parity filename updated

### 3. Delete File (Remove)
- ❌ **Not yet tested** - needs implementation
- Verifies: All 3 particles deleted, parity cleaned up

### 4. Move File (between directories/buckets)
- ❌ **Not yet tested** - needs implementation
- Verifies: All 3 particles moved, directory structure maintained

---

## 🎯 Test Scenarios

### Phase 1: Simple Cases (All Backends Available) ✅

**Create File**:
- ✅ Normal file creation (already in TestStandard)
- ✅ Empty file creation
- ✅ Large file creation (already in TestLargeDataQuick)

**Rename File**:
- ⏳ Rename within same directory
- ⏳ Rename to different directory
- ⏳ Rename with parity filename correctly updated (.parity-el/.parity-ol)
- ⏳ Rename empty file
- ⏳ Rename large file

**Delete File**:
- ⏳ Delete normal file (all 3 particles)
- ⏳ Delete file in degraded state (missing one particle)
- ⏳ Delete empty file
- ⏳ Delete non-existent file (should succeed or return error?)

**Move File**:
- ⏳ Move within same backend (same root)
- ⏳ Move to different directory
- ⏳ Move to different bucket/container
- ⏳ Move with parity filename correctly updated

---

### Phase 2: Error Cases (Need Discussion) ⚠️

#### Case (a): One Storage System Not Reachable

**Scenario**: One backend (even, odd, or parity) is completely unavailable
(e.g., network down, service crashed).

**Questions to Discuss**:

1. **Create File (Put)**:
   - Should Put fail immediately if one backend is down?
   - Or should it succeed on available backends and queue for later?
   - Current behavior: Likely fails (all 3 must succeed)
   - **Decision needed**: Fail fast or partial success?

2. **Rename File (Move)**:
   - If even backend is down but odd+parity succeed:
     - Should move succeed partially?
     - Should it rollback successful moves?
     - Should it return error but leave system in inconsistent state?
   - **Decision needed**: Atomic move or allow partial success?

3. **Delete File (Remove)**:
   - If one backend is down:
     - Should delete succeed if 2 of 3 succeed?
     - Should it return error if any fail?
     - Should it continue and log warnings?
   - Current behavior: Ignores "not found" errors
   - **Decision needed**: Require all 3 or allow partial deletion?

4. **Move File**:
   - Same questions as Rename
   - **Decision needed**: Atomic or partial?

---

#### Case (b): One Particle Doesn't Exist or Can't Be Moved/Created

**Scenario**: One particle is missing (corrupted, deleted, or never created),
but the backend itself is reachable.

**Questions to Discuss**:

1. **Create File (Put)**:
   - This shouldn't happen (new file), but if it does:
     - Should Put fail if it can't create all 3 particles?
     - **Decision**: Fail (ensures consistency)

2. **Rename File (Move)**:
   - If even particle exists but odd particle missing:
     - Should move succeed (move what exists)?
     - Should it fail (require all particles)?
     - Should it reconstruct missing particle first?
   - **Decision needed**: Require all particles or allow partial move?

3. **Delete File (Remove)**:
   - Current behavior: Ignores "not found" errors ✅
   - If particle missing: Delete succeeds (works correctly)
   - **Decision**: Current behavior is good, no change needed?

4. **Move File**:
   - If source particle missing:
     - Should move fail (can't move non-existent file)?
     - Or should it move what exists (degraded move)?
   - **Decision needed**: Require all source particles or allow partial?

---

## ✅ DECIDED: Hardware RAID 3 Compliant (Option A - Strict)

**Decision Date**: November 2, 2025

### Official Error Handling Policy:

**Reads**: Best effort (work with 2 of 3 backends) ✅
- Automatic reconstruction from parity
- Self-healing in background
- Transparent to users

**Writes**: Strict (require all 3 backends) ❌
- Put, Update, Move fail if any backend unavailable
- Prevents creating degraded files
- Matches hardware RAID 3 behavior

**Deletes**: Best effort (idempotent) ✅
- Succeed if any backends reachable
- Ignore "not found" errors
- Safe for cleanup operations

### Rationale:

**Hardware RAID 3 Compliance**:
- Industry standard: All hardware RAID 3 controllers block writes in degraded mode
- Linux MD RAID default behavior
- Proven approach over 30+ years

**Data Safety**:
- Prevents creating partially-written files
- No inconsistent states
- Every file is fully replicated or not created

**Simplicity**:
- No complex rollback logic for partial writes
- Clear error messages
- Predictable behavior

---

## 📝 Proposed Test Implementation Order

### Step 1: Simple Cases (Phase 1) ✅

Implement tests for normal operation:
1. ✅ Create - Already covered
2. ⏳ Rename - Add tests
3. ⏳ Delete - Add tests
4. ⏳ Move - Add tests

**Target**: All simple cases working before discussing error cases

---

### Step 2: Discussion (Phase 2)

Discuss and decide:
1. Behavior when one backend unavailable (a)
2. Behavior when one particle missing (b)
3. Error handling strategy
4. Rollback mechanisms (if needed)

---

### Step 3: Error Case Tests (Phase 3)

Implement tests based on decisions:
1. Mock unavailable backends
2. Simulate missing particles
3. Test error paths
4. Test rollback (if atomic)

---

## 📋 Test Structure

### Example: Rename Test Structure

```go
// TestRenameFile tests file renaming within the same directory.
//
// Renaming a file in level3 must rename all three particles (even, odd, parity)
// and update the parity filename suffix if the original file's length type changes.
//
// This test verifies:
//   - All three particles are renamed correctly
//   - Parity filename suffix is preserved (.parity-el or .parity-ol)
//   - Original file no longer exists at old location
//   - New file exists at new location with correct data
//
// Failure indicates: Rename operation doesn't maintain RAID 3 consistency.
func TestRenameFile(t *testing.T) {
    // Setup: Create file
    // Test: Rename file
    // Verify: All particles renamed, data intact
}
```

---

## 🎯 Next Steps

1. ✅ **Now**: Implement Phase 1 tests (simple cases)
2. ⏳ **Next**: Discuss Phase 2 error scenarios
3. ⏳ **After**: Implement Phase 2 tests based on decisions

---

**Ready to proceed with Phase 1 test implementation?** 🚀

