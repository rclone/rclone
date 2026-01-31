# Error Handling Policy - RAID 3 Compliance

**Date**: November 2, 2025  
**Decision**: Hardware RAID 3 Compliant (Strict Mode)

---

## 🎯 Official Policy

The raid3 backend follows hardware RAID 3 error handling behavior: read operations are supported in degraded mode with best effort (2 of 3 backends, safe read-only with automatic reconstruction), while write operations (Put, Update, Move) and delete operations (Remove, Rmdir, Purge, CleanUp) are blocked in degraded mode with strict policy (all 3 backends required) to prevent creating degraded files or partial deletes and ensure consistency.

---

## 📚 Rationale

### Hardware RAID 3 Compliance

This matches the industry standard: all hardware RAID 3 controllers block writes in degraded mode, matching Linux MD RAID default behavior and ZFS RAID-Z behavior. This is a proven approach used for 30+ years. The hardware RAID 3 specification requires reads to work with N-1 drives while writes require all N drives.

### Data Consistency

Strict writes prevent degraded file creation by ensuring every file is either fully replicated or not created, eliminating partial states or inconsistencies and preventing corruption from partial updates on retries.

### Performance

Strict writes avoid reconstruction overhead: new files don't need parity reconstruction, reconstruction is only needed for pre-existing degraded files, and heal handles rebuild in background.

### Simplicity

The approach provides clear error messages, requires no complex rollback logic for Put (errgroup handles it), and rollback is implemented for Move operations.

---

## 🔄 Behavior by Operation

### Read Operations ✅

Read operations work with ANY 2 of 3 backends available, provide automatic reconstruction from parity, restore missing particles in background via heal, and achieve performance of 6-7 seconds (S3 aggressive mode).

### Write Operations ❌

Put, Update, and Move operations require ALL 3 backends available, perform a pre-flight health check before each write (5-second timeout, +0.2s overhead), fail immediately if any backend is unavailable with clear error `"write blocked in degraded mode (RAID 3 policy)"`, and provide automatic rollback when `rollback=true` (default). Rollback status: Put rollback is fully working, Move rollback is fully working, but Update rollback is not working properly (see Known Limitations).

### Delete Operations ❌

Remove, Rmdir, Purge, and CleanUp operations require ALL 3 backends available, perform a pre-flight health check before each delete (5-second timeout, +0.2s overhead), fail immediately if any backend is unavailable with clear error `"delete blocked in degraded mode (RAID 3 policy)"`, and prevent partial deletes that could leave the system in an inconsistent state.

---

## 🛡️ Implementation Details

### Health Check Mechanism

Before every write operation, the backend performs a pre-flight health check that tests all 3 backends with parallel `List()` operations, uses a 5-second timeout per backend, returns an error immediately if any backend is unavailable, and prevents rclone's retry logic from creating partial files.

### Rollback Mechanism

When `rollback=true` (default), write operations provide an all-or-nothing guarantee: Put tracks successfully uploaded particles and removes all on failure, Move tracks successfully moved particles and moves back on failure, and Update uses move-to-temp pattern (currently has issues - see Known Limitations).

---

## 📊 Comparison

Before the fix, Put operations in degraded mode created degraded files on retry, Update operations corrupted files, Move operations created degraded files on retry, and there was a risk of corruption. After the fix, Put operations fail fast with clear error, Update operations preserve the original file, Move operations keep files at the original location, and data safety is guaranteed.

---

## ✅ Implementation Status

Already compliant: read operations (NewObject, Open work with 2/3), delete operations (strict policy, require all 3 backends), health check implemented, and Put/Move rollback implemented. Known issues: Update rollback not working properly (see [`OPEN_QUESTIONS.md`](OPEN_QUESTIONS.md) Q1).

---

## Related Documentation

- [`STRICT_WRITE_POLICY.md`](STRICT_WRITE_POLICY.md) - User-facing error handling guide
- [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md) - DD-001: Hardware RAID 3 Compliance decision record
- [`OPEN_QUESTIONS.md`](OPEN_QUESTIONS.md) - Q1: Update Rollback Not Working Properly

---
