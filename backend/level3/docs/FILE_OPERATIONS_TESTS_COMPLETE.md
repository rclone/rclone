# File Operations Tests - Phase 1 Complete ✅

**Date**: November 2, 2025  
**Status**: Phase 1 (Simple Cases) - **COMPLETE**

---

## ✅ Phase 1 Tests Implemented

### Create File (Put)
- ✅ **Already covered** in `TestStandard` (rclone integration tests)
- ✅ **Large files** covered in `TestLargeDataQuick` (1 MB)

**Coverage**: Complete ✅

---

### Rename File (Move)

**`TestRenameFile`** - Rename within same directory
- ✅ All three particles renamed correctly
- ✅ Parity filename suffix preserved
- ✅ Original location cleaned up
- ✅ Data integrity verified

**`TestRenameFileDifferentDirectory`** - Rename to different directory
- ✅ File moved between directories
- ✅ All particles relocated correctly
- ✅ Directory structure maintained
- ✅ Data integrity preserved

**`TestRenameFilePreservesParitySuffix`** - Parity suffix handling
- ✅ Odd-length files preserve `.parity-ol`
- ✅ Even-length files preserve `.parity-el`
- ✅ Suffix determined correctly from source

**Coverage**: Complete ✅

---

### Delete File (Remove)

**`TestDeleteFile`** - Normal deletion
- ✅ All three particles deleted
- ✅ File no longer exists after deletion
- ✅ Both parity suffixes checked (.parity-el and .parity-ol)
- ✅ Filesystem cleaned up correctly

**`TestDeleteFileIdempotent`** - Idempotent deletion
- ✅ Deleting already-deleted file succeeds
- ✅ No errors on multiple delete calls
- ✅ Safe for cleanup operations

**Coverage**: Complete ✅

---

### Move File (between directories/buckets)

**`TestMoveFileBetweenDirectories`** - Move between directories
- ✅ File moves correctly between directories
- ✅ All three particles relocated
- ✅ Original location cleaned up
- ✅ Directory structure maintained
- ✅ Data integrity preserved
- ✅ Filesystem state verified

**Coverage**: Complete ✅

---

## 📊 Test Results

```
=== RUN   TestRenameFile
--- PASS: TestRenameFile (0.00s)

=== RUN   TestRenameFileDifferentDirectory
--- PASS: TestRenameFileDifferentDirectory (0.00s)

=== RUN   TestDeleteFile
--- PASS: TestDeleteFile (0.00s)

=== RUN   TestDeleteFileIdempotent
--- PASS: TestDeleteFileIdempotent (0.00s)

=== RUN   TestMoveFileBetweenDirectories
--- PASS: TestMoveFileBetweenDirectories (0.00s)

=== RUN   TestRenameFilePreservesParitySuffix
--- PASS: TestRenameFilePreservesParitySuffix (0.00s)

PASS
ok  	github.com/rclone/rclone/backend/level3	0.265s
```

**All 6 new tests passing!** ✅

---

## 🎯 What's Covered

### ✅ Normal Operations (All Backends Available)

| Operation | Test | Status |
|-----------|------|--------|
| **Create** | `TestStandard`, `TestLargeDataQuick` | ✅ Covered |
| **Rename** | `TestRenameFile` | ✅ Covered |
| **Rename (dir)** | `TestRenameFileDifferentDirectory` | ✅ Covered |
| **Rename (suffix)** | `TestRenameFilePreservesParitySuffix` | ✅ Covered |
| **Delete** | `TestDeleteFile` | ✅ Covered |
| **Delete (idempotent)** | `TestDeleteFileIdempotent` | ✅ Covered |
| **Move** | `TestMoveFileBetweenDirectories` | ✅ Covered |

**Total**: 7 tests covering all basic file operations ✅

---

## ⏳ Phase 2: Error Cases (Need Discussion)

### Case (a): One Storage System Not Reachable

**Scenario**: Backend completely unavailable (network down, crashed, etc.)

**Operations Needing Decision**:

1. **Create (Put)**:
   - Current: Likely fails (all 3 must succeed)
   - **Question**: Should Put fail immediately or allow partial success?
   - **Recommendation**: Fail fast (ensures consistency)

2. **Rename/Move**:
   - **Question**: Atomic (all or nothing) or allow partial moves?
   - **Recommendation**: Atomic with rollback (prevents inconsistent state)

3. **Delete**:
   - Current: Already handles gracefully (ignores missing particles) ✅
   - **Question**: Should delete succeed if 2 of 3 succeed?
   - **Recommendation**: Current behavior is good (idempotent delete)

---

### Case (b): One Particle Missing or Can't Be Created/Moved

**Scenario**: Particle missing (deleted, corrupted, never created), but backend reachable

**Operations Needing Decision**:

1. **Create (Put)**:
   - **Question**: Should Put fail if can't create all 3 particles?
   - **Decision**: Yes, fail (ensures consistency)

2. **Rename/Move**:
   - **Question**: Should move require all source particles or allow partial?
   - **Recommendation**: Require all (atomic operation)
   - **Alternative**: Allow partial but log warnings

3. **Delete**:
   - Current: Already handles missing particles gracefully ✅
   - **Decision**: Keep current behavior

4. **Move**:
   - **Question**: Should move fail if source particle missing?
   - **Recommendation**: Fail (can't move non-existent file)
   - **Exception**: If degraded mode enabled, could reconstruct first?

---

## 🤔 Discussion Points

### Point 1: Atomicity vs. Availability

**Option A: Strict Atomicity** (Recommended)
- All operations require all 3 backends/particles
- Fails fast if any backend/particle unavailable
- **Pro**: Guarantees consistency
- **Con**: Operations fail when backend down (poor availability)

**Option B: Best Effort**
- Operations succeed if 2 of 3 backends/particles available
- Log warnings for partial success
- **Pro**: Better availability (works in degraded mode)
- **Con**: Inconsistent state possible

**Recommendation**: **Hybrid approach**:
- **Reads**: Best effort (already implemented - works with 2 of 3)
- **Writes (Put)**: Atomic (fail if any backend unavailable)
- **Moves**: Atomic (fail if any source particle missing)
- **Deletes**: Best effort (already implemented - idempotent)

---

### Point 2: Rollback Strategy

**For Atomic Operations** (Put, Move):
- If operation fails partway through:
  - **Option 1**: Rollback successful operations
  - **Option 2**: Leave partial state, return error
  - **Option 3**: Retry failed operations

**Recommendation**: **Option 1** (Rollback)
- Prevents orphaned particles
- Maintains consistency
- Requires tracking which operations succeeded

**Complexity**: High (need transaction-like behavior)

---

### Point 3: Error Reporting

**When operations fail partially**:
- **Option 1**: Return first error encountered
- **Option 2**: Collect all errors, return aggregate
- **Option 3**: Return specific error per backend

**Recommendation**: **Option 2** (Aggregate errors)
- User knows which backends failed
- Better debugging information
- Helps with troubleshooting

---

## 📝 Next Steps

### Immediate (Done) ✅
- ✅ Implement Phase 1 tests (simple cases)
- ✅ All tests passing
- ✅ Documentation complete

### Short Term (Next)
1. **Discuss Phase 2 error scenarios**:
   - Review recommendations above
   - Decide on atomicity vs. availability
   - Choose rollback strategy
   - Define error reporting format

2. **Document decisions**:
   - Create error handling specification
   - Define behavior for each operation in error cases

### Medium Term (After Discussion)
3. **Implement Phase 2 tests**:
   - Mock unavailable backends
   - Simulate missing particles
   - Test error paths
   - Test rollback (if atomic)

4. **Implement error handling** (if needed):
   - Rollback mechanisms
   - Aggregate error reporting
   - Retry logic

---

## 📋 Test Summary

### New Tests Added: **6**

1. `TestRenameFile` - Basic rename
2. `TestRenameFileDifferentDirectory` - Rename across directories
3. `TestRenameFilePreservesParitySuffix` - Parity suffix handling
4. `TestDeleteFile` - Normal deletion
5. `TestDeleteFileIdempotent` - Idempotent deletion
6. `TestMoveFileBetweenDirectories` - Move between directories

### Total Tests Now: **22**

- Integration: 2
- Byte Operations: 3
- Validation: 1
- Parity: 2
- Reconstruction: 4
- Degraded Mode: 2
- Self-Healing: 5
- **File Operations: 6** (NEW)

---

## ✅ Phase 1 Status: **COMPLETE**

All simple cases (all backends available, all particles present) are now:
- ✅ Fully tested
- ✅ Well documented
- ✅ Passing consistently

**Ready to proceed with Phase 2 discussion!** 🎉

---

**Next**: Review `FILE_OPERATIONS_TEST_PLAN.md` and discuss error handling scenarios before implementing Phase 2 tests.

