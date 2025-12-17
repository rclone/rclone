# Error Handling Policy - RAID 3 Compliance

**Date**: November 2, 2025  
**Decision**: Hardware RAID 3 Compliant (Strict Mode)

---

## 🎯 Official Policy

The raid3 backend follows **hardware RAID 3 error handling behavior**:

| Operation | Degraded Mode | Policy | Rationale |
|-----------|---------------|--------|-----------|
| **Read** | ✅ Supported | Best effort (2 of 3) | Safe (read-only), automatic reconstruction |
| **Write** (Put, Update, Move) | ❌ Blocked | Strict (all 3) | Prevents creating degraded files, ensures consistency |
| **Delete** (Remove, Rmdir, Purge, CleanUp) | ❌ Blocked | Strict (all 3) | Prevents partial deletes, ensures consistency |

---

## 📚 Rationale

### Hardware RAID 3 Compliance

**Industry Standard**:
- All hardware RAID 3 controllers block writes in degraded mode
- Linux MD RAID default behavior
- ZFS RAID-Z behavior
- Proven approach for 30+ years

**Reference**: Hardware RAID 3 specification
- Reads: Work with N-1 drives ✅
- Writes: Require all N drives ❌

### Data Consistency

**Prevents Degraded File Creation**:
- Every file is either fully replicated OR not created
- No partial states or inconsistencies
- Prevents corruption from partial updates on retries

### Performance

**Avoids Reconstruction Overhead**:
- New files don't need parity reconstruction
- Reconstruction only for pre-existing degraded files
- Heal handles rebuild in background

### Simplicity

- Clear error messages
- No complex rollback logic needed for Put (errgroup handles it)
- Rollback implemented for Move operations

---

## 🔄 Behavior by Operation

### Read Operations ✅

- Work with ANY 2 of 3 backends available
- Automatic reconstruction from parity
- Heal restores missing particles in background
- Performance: 6-7 seconds (S3 aggressive mode)

### Write Operations ❌

**Put, Update, Move**:
- Require ALL 3 backends available
- Pre-flight health check before each write (5-second timeout, +0.2s overhead)
- Fail immediately if any backend unavailable
- Clear error: `"write blocked in degraded mode (RAID 3 policy)"`
- Automatic rollback when `rollback=true` (default)

**Rollback Status**:
- ✅ Put rollback: Fully working
- ✅ Move rollback: Fully working
- ⚠️ Update rollback: Not working properly (see Known Limitations)

### Delete Operations ❌

**Remove, Rmdir, Purge, CleanUp**:
- Require ALL 3 backends available
- Pre-flight health check before each delete (5-second timeout, +0.2s overhead)
- Fail immediately if any backend unavailable
- Clear error: `"delete blocked in degraded mode (RAID 3 policy)"`
- Prevents partial deletes that could leave system in inconsistent state

---

## 🛡️ Implementation Details

### Health Check Mechanism

Before every write operation, the backend performs a pre-flight health check:
- Tests all 3 backends with parallel `List()` operations
- 5-second timeout per backend
- Returns error immediately if any unavailable
- Prevents rclone's retry logic from creating partial files

### Rollback Mechanism

When `rollback=true` (default), write operations provide an **all-or-nothing guarantee**:
- **Put**: Tracks successfully uploaded particles, removes all on failure
- **Move**: Tracks successfully moved particles, moves back on failure
- **Update**: Uses move-to-temp pattern (currently has issues - see Known Limitations)

---

## 📊 Comparison

| Aspect | Before Fix | After Fix |
|--------|------------|-----------|
| **Put (degraded)** | Created degraded file on retry | Fails fast with clear error |
| **Update (degraded)** | 🚨 Corrupted file | Original file preserved |
| **Move (degraded)** | Created degraded file on retry | File stays at original location |
| **Data Safety** | ⚠️ Risk of corruption | ✅ Guaranteed |

---

## ✅ Implementation Status

**Already Compliant**:
- ✅ Read operations (NewObject, Open work with 2/3)
- ✅ Delete operations (strict policy, require all 3 backends)
- ✅ Health check implemented
- ✅ Put/Move rollback implemented

**Known Issues**:
- ⚠️ Update rollback not working properly (see `OPEN_QUESTIONS.md` Q1)

---

## Related Documentation

- [`STRICT_WRITE.md`](STRICT_WRITE.md) - User-facing error handling guide
- [`DESIGN_DECISIONS.md`](../DESIGN_DECISIONS.md) - DD-001: Hardware RAID 3 Compliance decision record
- [`OPEN_QUESTIONS.md`](../OPEN_QUESTIONS.md) - Q1: Update Rollback Not Working Properly

---

**This makes raid3 a true, compliant RAID 3 implementation!** 🎯
