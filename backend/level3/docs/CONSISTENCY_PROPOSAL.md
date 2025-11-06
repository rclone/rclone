# Level3 Backend - Consistency Behavior Proposal

**Date**: November 5, 2025  
**Context**: Manual testing revealed inconsistent behavior when objects have incomplete particles  
**Status**: 🔬 **PROPOSAL** - Awaiting decision before implementation

---

## 🎯 Problem Statement

### Current Behavior Issues

When testing with 3 Minio instances, inconsistent behavior was observed:

**Scenario 1** (bucket exists in 2 remotes):
```bash
$ rclone purge miniolevel3:mybucket
# ✅ Works correctly, no error messages
```

**Scenario 2** (bucket exists in only 1 remote):
```bash
$ rclone purge miniolevel3:mybucket
# ⚠️ Gives lots of error messages
# ✅ But still removes the bucket
```

### Root Cause

The inconsistency stems from differing thresholds for operations:

| Operation | Current Behavior | Threshold |
|-----------|-----------------|-----------|
| **List()** | Shows ALL objects | 1+ particles present |
| **NewObject()** | Validates object integrity | 2+ particles present (RAID 3) |
| **Remove()** | Best-effort deletion | Works with any particles |
| **Open()** | Reconstructs data | 2+ particles present (RAID 3) |

**The Problem**:
- `List()` shows objects with only 1 particle (broken objects)
- `NewObject()` fails for objects with only 1 particle
- `rclone purge` calls `List()` then tries `NewObject()` → errors for broken objects
- But `Remove()` eventually succeeds anyway (best-effort)

---

## 🤔 Fundamental Questions

### Question 1: What is an "object" in level3?

**Options**:

**A. Strict RAID 3 Definition** (2+ particles):
- An object exists only if it can be reconstructed
- Objects with <2 particles are "corrupted" or "not exist"
- Consistent with RAID 3 specification

**B. Lenient Definition** (1+ particles):
- An object exists if any particle exists
- Objects with <2 particles are "broken but present"
- More forgiving for degraded scenarios

### Question 2: Should operations work on broken objects?

For each operation category:

| Operation Type | On Broken Objects (1 particle) | Rationale |
|---------------|-------------------------------|-----------|
| **Read** (copy, cat) | ? | Cannot reconstruct data |
| **Write** (put, update) | ? | Already blocked (requires all 3) |
| **Delete** (remove, purge) | ? | Should we clean up broken objects? |
| **List** | ? | Should broken objects be visible? |
| **Metadata** (stat, size) | ? | Can we report metadata without reconstruction? |

---

## 💡 Proposed Solution: "Strict RAID 3 with Silent Cleanup"

### Core Principle

**An object is valid if and only if it can be reconstructed (2+ particles present)**

This means:
- Objects are either **fully functional** or **don't exist** (from user's perspective)
- Broken objects (1 particle) are treated as **corrupted fragments to be cleaned up**
- Operations maintain RAID 3 guarantees while silently handling edge cases

### Detailed Behavior

#### 1. **List Operations** (ls, lsl, lsd, size)

**Behavior**: Show only reconstructable objects (2+ particles)

**Implementation**:
```go
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
    // Collect entries from all backends
    allEntries := mergeEntriesFromBackends()
    
    // Filter: only include objects with 2+ particles
    validEntries := []fs.DirEntry{}
    for _, entry := range allEntries {
        if isObject(entry) {
            if f.hasMinimumParticles(entry, 2) {
                validEntries = append(validEntries, entry)
            }
            // Silently skip broken objects (1 particle)
        } else {
            // Always include directories
            validEntries = append(validEntries, entry)
        }
    }
    return validEntries, nil
}
```

**Result**:
- ✅ Users only see valid objects
- ✅ No confusing "object exists but can't be read" scenarios
- ✅ Consistent with RAID 3 semantics

---

#### 2. **Read Operations** (copy FROM, cat, open)

**Behavior**: Work with 2+ particles (already implemented correctly)

**Current implementation**: ✅ Already correct
- Reconstructs from any 2 of 3 particles
- Performs self-healing automatically
- Fails with clear error if <2 particles

**No changes needed**

---

#### 3. **Write Operations** (copy TO, put, update, move)

**Behavior**: Require all 3 backends available (already implemented correctly)

**Current implementation**: ✅ Already correct
- Pre-flight health check
- Blocks writes in degraded mode
- Clear error messages with recovery guidance

**No changes needed**

---

#### 4. **Delete Operations** (remove, purge, rmdir)

**Behavior**: Best-effort cleanup with silent broken object handling

**Implementation**:
```go
func (o *Object) Remove(ctx context.Context) error {
    // Try to remove from all backends
    // Ignore "not found" errors (idempotent)
    // Succeed if ANY deletion succeeds OR all report "not found"
    
    g, gCtx := errgroup.WithContext(ctx)
    
    g.Go(func() error {
        obj, err := o.fs.even.NewObject(gCtx, o.remote)
        if err != nil {
            return nil // Ignore if not found
        }
        return obj.Remove(gCtx)
    })
    
    // ... odd, parity ...
    
    err := g.Wait()
    
    // Success if at least one backend confirmed deletion
    // (even if others failed with "not found")
    return err
}
```

**Special Cases**:

**A. Broken Objects (1 particle)**:
- `List()` won't show them (filtered out)
- Direct `Remove()` call should succeed (cleanup)
- No error messages (silent cleanup)

**B. Degraded Mode (1 backend down, object in 2+ remotes)**:
- Deletion succeeds on available backends
- Missing backend: "not found" error ignored
- Object successfully removed from user's perspective

**C. Purge Operations**:
- Lists only valid objects (2+ particles)
- Deletes all listed objects
- No errors for broken objects (they're not listed)

---

#### 5. **Metadata Operations** (stat, size, hash)

**Behavior**: Work only with reconstructable objects (2+ particles)

**Reasoning**:
- Can't reliably determine size/hash without reconstruction
- Consistency with "object exists = can be read" principle
- Metadata without data is misleading

---

### Error Messages

#### For Users

**Broken Object Cleanup** (background):
```
# No user-visible messages
# Broken objects are invisible and cleaned up silently
```

**Valid Object in Degraded Mode**:
```
$ rclone copy miniolevel3:mybucket/file.txt local:
# ✅ Works silently (reconstructs from 2/3 particles)
# Maybe: INFO message about self-healing
2024/11/05 10:30:15 INFO  : file.txt: Self-healed (restored from parity)
```

**Write in Degraded Mode**:
```
$ rclone copy local:file.txt miniolevel3:mybucket/
ERROR : cannot upload files - level3 backend is DEGRADED (1 of 3 backends unavailable)
NOTICE: RAID 3 Policy: Writes require ALL backends healthy
NOTICE: To recover: Fix the unavailable backend, then retry
NOTICE: Unavailable backends: [odd]
```

#### For Administrators

**Debug Logging** (with -vv):
```
2024/11/05 10:30:15 DEBUG : List: Found file.txt with 1 particle (broken) - skipping
2024/11/05 10:30:15 DEBUG : List: Found otherfile.txt with 2 particles (valid) - including
```

**Cleanup Operations** (optional rclone check --cleanup):
```
$ rclone check miniolevel3:mybucket --cleanup
Checking mybucket...
Found 3 broken objects (1 particle each):
  - file1.txt (only in even)
  - file2.txt (only in odd)
  - file3.txt (only in parity)
Cleaned up 3 broken objects
```

---

## 📊 Comparison: Before vs After

### Scenario: Bucket with 2 valid files + 1 broken file

**Before** (current behavior):
```bash
$ rclone ls miniolevel3:mybucket
     1024 file1.txt          # Valid (2 particles)
     2048 file2.txt          # Valid (2 particles)
      512 broken.txt         # Broken (1 particle) ⚠️

$ rclone cat miniolevel3:mybucket/broken.txt
ERROR: Cannot reconstruct broken.txt

$ rclone purge miniolevel3:mybucket
ERROR: Cannot find object broken.txt
ERROR: Failed to delete broken.txt
# ⚠️ Confusing - it was listed but can't be deleted?
# ✅ Bucket eventually purged
```

**After** (proposed behavior):
```bash
$ rclone ls miniolevel3:mybucket
     1024 file1.txt          # Valid (2 particles)
     2048 file2.txt          # Valid (2 particles)
# broken.txt not listed (filtered out)

$ rclone cat miniolevel3:mybucket/broken.txt
ERROR: object not found

$ rclone purge miniolevel3:mybucket
# ✅ Works silently (deletes valid files, ignores broken fragments)
# Broken fragments cleaned up in background
```

---

## 🎯 Consistency Rules Summary

### The Three Principles

**1. Visibility Principle**
> Objects are visible if and only if they are functional

- List shows only objects with 2+ particles
- Broken objects (1 particle) are invisible
- This prevents "exists but unusable" confusion

**2. Integrity Principle**
> Operations succeed or fail based on RAID 3 guarantees

- Reads require 2+ particles (can reconstruct)
- Writes require 3 backends (all healthy)
- Deletes are best-effort (cleanup role)

**3. Silent Cleanup Principle**
> Broken fragments are cleaned up without user intervention

- Delete operations clean up ANY fragments found
- No error messages for broken objects
- Automatic cleanup during normal operations

---

## 🔧 Implementation Changes Required

### 1. List() Enhancement

**Current**:
```go
// Shows all objects from any backend
entryMap[entry.Remote()] = entry
```

**Proposed**:
```go
// Filter objects based on particle count
if isObject(entry) {
    particleCount := f.countParticles(ctx, entry.Remote())
    if particleCount >= 2 {
        entryMap[entry.Remote()] = entry
    }
    // Silently skip objects with <2 particles
}
```

**Performance**: Adds particle counting overhead to List operations
- **Mitigation**: Cache particle counts, parallel checks

---

### 2. Remove() Enhancement

**Current**: ✅ Already mostly correct
- Ignores "not found" errors
- Uses errgroup for parallel deletion

**Proposed**: No changes needed
- Already implements best-effort deletion
- Already idempotent

---

### 3. NewObject() Verification

**Current**: ✅ Already correct
- Requires 2+ particles
- Returns "not found" for broken objects

**Proposed**: No changes needed

---

### 4. Optional: Cleanup Command

**New feature** for administrators:

```bash
$ rclone cleanup miniolevel3:mybucket
Scanning for broken objects...
Found 5 broken objects:
  - file1.txt (1 particle in even)
  - file2.txt (1 particle in odd)
  - file3.txt (1 particle in parity)
  - ...
Cleaned up 5 broken objects (freed 2.5 MB)
```

---

## 🤔 Alternative Approaches Considered

### Alternative 1: "Lenient Mode" (Show All, Warn on Broken)

**Idea**: List shows all objects, but marks broken ones

```bash
$ rclone ls miniolevel3:mybucket
     1024 file1.txt
     2048 file2.txt
      512 broken.txt    [BROKEN - 1 particle]
```

**Pros**:
- ✅ Users see all particles (transparency)
- ✅ Can manually clean up broken objects

**Cons**:
- ❌ Confusing UX ("file exists but can't read it?")
- ❌ Breaks normal rclone workflows (copy/sync fail on broken files)
- ❌ Not RAID 3 compliant (RAID doesn't show corrupted blocks)

**Decision**: ❌ Rejected (violates RAID 3 semantics)

---

### Alternative 2: "Strict Mode" (Fail on Any Broken Objects)

**Idea**: Operations fail if any broken objects detected

```bash
$ rclone purge miniolevel3:mybucket
ERROR: Cannot purge - bucket contains 1 broken object
Please run: rclone cleanup miniolevel3:mybucket first
```

**Pros**:
- ✅ Forces administrators to fix issues
- ✅ Very explicit about problems

**Cons**:
- ❌ Annoying for users (extra steps required)
- ❌ Broken objects from past failures block current operations
- ❌ Not resilient (RAID should handle failures gracefully)

**Decision**: ❌ Rejected (too strict, poor UX)

---

### Alternative 3: "Automatic Healing" (Reconstruct and Restore)

**Idea**: Automatically restore missing particles during List/Purge

```bash
$ rclone purge miniolevel3:mybucket
INFO: Self-healing 1 broken object...
INFO: Restored broken.txt (recreated missing particles)
# Then proceeds with purge
```

**Pros**:
- ✅ Fixes broken objects automatically
- ✅ Very resilient

**Cons**:
- ❌ Can't reconstruct with only 1 particle (need 2+)
- ❌ Would slow down List operations significantly
- ❌ Purge should delete, not heal

**Decision**: ❌ Rejected (not applicable for 1-particle objects)

---

## ✅ Recommended Solution

### Chosen Approach: "Strict RAID 3 with Silent Cleanup"

**Why**:
1. ✅ **RAID 3 Compliant**: Matches hardware RAID 3 behavior
2. ✅ **Consistent**: "Object exists = object is readable" invariant
3. ✅ **Clean UX**: No confusing error messages for users
4. ✅ **Resilient**: Silent cleanup prevents fragment accumulation
5. ✅ **Correct**: Operations have clear success/failure semantics

**Implementation Complexity**: Low
- Only List() needs modification (add particle counting)
- Other operations already correct or nearly correct
- Estimated effort: 4-6 hours

---

## 📋 Testing Strategy

### Test Cases to Add

1. **List with broken objects**:
   - Create objects, manually delete particles
   - Verify List excludes broken objects
   - Verify List includes valid objects

2. **Purge with broken objects**:
   - Create bucket with mix of valid and broken objects
   - Verify purge succeeds without errors
   - Verify all particles cleaned up (valid + broken)

3. **Delete broken object directly**:
   - Create broken object (1 particle)
   - Call Remove() directly
   - Verify succeeds without error
   - Verify particle cleaned up

4. **Copy from broken object**:
   - Create broken object (1 particle)
   - Try to copy from it
   - Verify fails with "object not found" (not "can't reconstruct")

5. **Mixed scenarios**:
   - Degraded mode (1 backend down) + broken objects
   - Verify correct behavior in all cases

---

## 🎯 Decision Request

### Questions for User

Before implementing, please decide:

1. **Should List() hide broken objects (1 particle)?**
   - [ ] Yes - strict RAID 3 (recommended)
   - [ ] No - show all objects with warnings

2. **Should delete operations clean up broken objects silently?**
   - [ ] Yes - best-effort cleanup (recommended)
   - [ ] No - fail on broken objects

3. **Should we add a dedicated cleanup command?**
   - [ ] Yes - `rclone cleanup` for manual broken object removal
   - [ ] No - automatic cleanup is sufficient
   - [ ] Maybe later - not priority

4. **Should we log broken object detection?**
   - [ ] Yes, always (INFO level)
   - [ ] Only in verbose mode (-v or -vv)
   - [ ] Only in debug mode (-vv)
   - [ ] No - completely silent (recommended for normal operations)

---

## 📝 Implementation Plan

If approved, implementation order:

1. **Add particle counting helper** (1 hour)
   - `func (f *Fs) countParticles(ctx, remote) int`
   - Efficient parallel checking

2. **Modify List() to filter** (2 hours)
   - Filter objects with <2 particles
   - Add debug logging for broken objects
   - Maintain performance

3. **Add tests** (2 hours)
   - 5 test cases from testing strategy
   - Edge cases and degraded mode

4. **Documentation** (1 hour)
   - Update README with behavior
   - Update error handling docs
   - Add troubleshooting guide

**Total Estimated Effort**: 6 hours

---

## 🎉 Expected Outcome

After implementation:

**User Experience**:
- ✅ `rclone purge` works consistently (no unexpected errors)
- ✅ `rclone ls` shows only usable objects
- ✅ `rclone copy` fails cleanly for truly missing objects
- ✅ Clear mental model: "if I see it, I can use it"

**Administrator Experience**:
- ✅ Broken objects cleaned up automatically
- ✅ Debug logs show what's happening
- ✅ Optional manual cleanup command
- ✅ No fragment accumulation over time

**RAID 3 Compliance**:
- ✅ All operations follow RAID 3 semantics
- ✅ Consistent with hardware RAID behavior
- ✅ Clear documentation of behavior
- ✅ Predictable and testable

---

**End of Proposal**

*Awaiting user decision on the 4 questions above before proceeding with implementation.*

