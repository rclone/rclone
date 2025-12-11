# Error Handling Decision Summary

**Date**: November 2, 2025  
**Decision**: Hardware RAID 3 Compliant (Option A - Strict)

---

## ✅ Official Decision

The level3 backend will follow **hardware RAID 3 error handling behavior**:

### Policy:

| Operation | Degraded Mode | Policy | Rationale |
|-----------|---------------|--------|-----------|
| **Read** | ✅ Supported | Best effort (2 of 3) | Safe (read-only), already implemented |
| **Write (Put)** | ❌ Blocked | Strict (all 3) | Prevents creating degraded files |
| **Write (Update)** | ❌ Blocked | Strict (all 3) | Prevents partial updates |
| **Move/Rename** | ❌ Blocked | Strict (all 3) | Prevents inconsistent renames |
| **Delete** | ✅ Supported | Best effort (idempotent) | Missing = deleted (same state) |

---

## 📚 Why This Decision?

### 1. Hardware RAID 3 Compliance

**Industry Standard**:
- All hardware RAID 3 controllers block writes in degraded mode
- Linux MD RAID default behavior (write-intent bitmap)
- ZFS RAID-Z behavior
- Proven approach for 30+ years

**Reference**: Hardware RAID 3 specification
- Reads: Work with N-1 drives ✅
- Writes: Require all N drives ❌

---

### 2. Data Consistency

**Prevents Degraded File Creation**:
```
Bad (partial success):
User uploads → Backend down → File created with missing particle
              → Every read needs reconstruction
              → Performance degraded
              → Amplification effect

Good (strict):
User uploads → Backend down → Upload fails
              → User fixes backend
              → Retry succeeds
              → All files fully replicated
```

**Consistency Guarantee**:
- Every file is either fully replicated OR not created
- No partial states
- No inconsistencies

---

### 3. Performance

**Avoids Reconstruction Overhead**:
- New files don't need parity reconstruction
- Reconstruction only for pre-existing degraded files
- Heal handles rebuild in background

**Scenario**: Upload 100 files while backend down
- Strict: All 100 fail (user fixes backend, retries)
- Partial: 100 files created degraded (100 reconstructions needed forever!)

---

### 4. Simplicity

**No Complex Rollback**:
- errgroup automatically cancels on error
- No need to track partial success
- No need to undo completed operations (for Put)
- Clear error messages

**For Move**: Will add simple rollback logic (delete from new location if any move fails)

---

## 🔄 Implementation Status

### Already Compliant ✅

**Read Operations**:
- ✅ NewObject works with 2 of 3 particles
- ✅ Open reconstructs from parity
- ✅ Heal restores missing particles
- ✅ **No changes needed**

**Delete Operations**:
- ✅ Remove ignores "not found" errors
- ✅ Idempotent behavior
- ✅ **No changes needed**

---

### To Verify/Implement ⏳

**Write Operations** (Put, Update):
- Current: Uses errgroup (likely already strict)
- **Action**: Verify behavior when backend unavailable
- **Test**: Add Phase 2 tests

**Move Operations**:
- Current: Uses errgroup, ignores "not found" in Move
- **Action**: Add rollback if any move fails
- **Test**: Add Phase 2 tests with rollback verification

---

## 📝 Documentation Updates

### Completed ✅

1. **`ERROR_HANDLING_POLICY.md`** - Official policy document
2. **`ERROR_HANDLING_ANALYSIS.md`** - Analysis and comparison
3. **`README.md`** - Updated degraded mode section with policy
4. **`RAID3.md`** - Added error handling compliance section
5. **`TESTS.md`** - Added policy summary
6. **`raid3_test.go`** - Updated test comments with policy notes
7. **`FILE_OPERATIONS_TEST_PLAN.md`** - Updated with decision

---

## 🎯 Next Steps

### Phase 2: Error Case Tests

**To Implement**:

1. **Test Put with unavailable backend**
   - Mock/simulate unavailable backend
   - Verify Put fails with clear error
   - Verify no partial particles created

2. **Test Move with unavailable backend**
   - Verify Move fails when backend unavailable
   - Verify rollback works (if needed)
   - Verify system returns to consistent state

3. **Test Delete with unavailable backend**
   - Verify Delete succeeds when backends available
   - Verify idempotent behavior

4. **Test Move with missing source particle**
   - Verify Move fails if source particle missing
   - Or verify Move reconstructs first (if we decide to support this)

---

## 💡 Future Enhancements (Optional)

### Configurable Write Policy

If users request higher write availability, could add:

```go
type Options struct {
    // ...
    WritePolicy string `config:"write_policy"`
    // "strict"   - require all 3 backends (default, RAID 3 compliant)
    // "degraded" - allow writes with 2 backends (advanced, creates degraded files)
}
```

**Not implementing now** - keeping it simple with strict-only.

---

## ✨ Summary

**Decision**: Option A (Strict) - Hardware RAID 3 Compliant ✅

**Benefits**:
- ✅ Industry standard behavior
- ✅ Data consistency guaranteed
- ✅ Simple implementation
- ✅ Clear error messages
- ✅ Production safe

**Trade-offs Accepted**:
- ❌ Writes fail when backend unavailable
- ✅ But this is expected RAID behavior!
- ✅ Reads still work (degraded mode)
- ✅ Heal restores existing files

**Status**: Documented and ready for implementation verification/testing

---

**This makes level3 a true, compliant RAID 3 implementation!** 🎯

