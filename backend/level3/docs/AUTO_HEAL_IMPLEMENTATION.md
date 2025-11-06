# Auto-Heal Implementation - Automatic Reconstruction Feature

**Date**: November 6, 2025  
**Status**: ✅ **IMPLEMENTED AND TESTED**  
**Related**: RAID3_SEMANTICS_DISCUSSION.md

---

## Overview

Implemented separate `auto_heal` flag to control automatic reconstruction of degraded items (2/3 particles present), distinct from `auto_cleanup` which handles orphaned items (1/3 particles).

---

## Terminology

**Rebuild** (1rm):
- One entire remote unreachable/failed
- Command: `rclone backend rebuild level3:`
- Restores ALL particles on replacement backend

**Reconstruct** (0rm):
- All 3 remotes available
- Individual items have missing particles (2/3 present)
- Self-healing: automatic reconstruction during operations

**Cleanup** (0rm):
- Orphaned items with 1/3 particles
- Cannot reconstruct - hide/delete

---

## Configuration Flags

### auto_cleanup (existing)
**Purpose**: Handle orphaned items (1/3 particles - cannot reconstruct)

**Behavior**:
- `true` (default): Hide orphaned items from listings, delete when accessed
- `false`: Show orphaned items (debugging mode)

### auto_heal (NEW)
**Purpose**: Reconstruct degraded items (2/3 particles - can reconstruct)

**Behavior**:
- `true` (default): Automatically reconstruct missing particles/directories
- `false`: No automatic reconstruction (manual rebuild needed)

---

## Behavior Matrix

| auto_cleanup | auto_heal | 2/3 (Degraded) | 1/3 (Orphaned) | Use Case |
|--------------|-----------|----------------|----------------|----------|
| `true` | `true` | Reconstruct ✅ | Hide/Delete ✅ | **Production** (recommended) |
| `true` | `false` | No reconstruct ❌ | Hide/Delete ✅ | Conservative (no auto-healing) |
| `false` | `true` | Reconstruct ✅ | Show ⚠️ | Debugging with healing |
| `false` | `false` | No reconstruct ❌ | Show ⚠️ | **Raw debugging** mode |

---

## Operations Affected by auto_heal

### 1. List() - Directory Reconstruction (1dm)

**Scenario**: Directory exists on 2/3 backends

**auto_heal=true**:
```bash
$ rclone ls level3:mydir  # mydir on even+odd, missing on parity
# ✅ Lists contents
# ✅ Creates missing directory on parity
# LOG: "Reconstructing missing directory 'mydir' on parity backend (2/3 → 3/3)"
```

**auto_heal=false**:
```bash
$ rclone ls level3:mydir  # mydir on even+odd, missing on parity
# ✅ Lists contents
# ❌ Does NOT create missing directory
# Parity backend still missing directory
```

**Implementation**: `reconstructMissingDirectory()` function in `List()`

---

### 2. Open() - File Particle Reconstruction (1fm)

**Scenario**: File exists with 2/3 particles

**auto_heal=true**:
```bash
$ rclone cat level3:file.txt  # file has even+odd, missing parity
# ✅ Reconstructs from even+odd
# ✅ Queues parity particle for background upload
# LOG: "Reconstructed file.txt from even+odd (degraded mode)"
# LOG: "Queued parity particle for self-healing upload"
```

**auto_heal=false**:
```bash
$ rclone cat level3:file.txt  # file has even+odd, missing parity
# ✅ Reconstructs from even+odd (still works!)
# ❌ Does NOT queue self-healing upload
# LOG: "Reconstructed file.txt from even+odd (degraded mode)"
# No queueing message
```

**Implementation**: Check `auto_heal` before calling `queueParticleUpload()` in `Open()`

---

### 3. DirMove() - Best-Effort Move with Reconstruction (1dm)

**Scenario**: Directory exists on 2/3 backends, attempting to rename

**auto_heal=true**:
```bash
$ rclone moveto level3:mydir level3:mydir2  # mydir on even+odd, missing on parity
# ✅ Moves mydir→mydir2 on even and odd
# ✅ Creates mydir2 on parity (reconstruction)
# LOG: "DirMove: source missing on parity, creating destination (reconstruction)"
# Result: mydir2 exists on all 3 backends
```

**auto_heal=false**:
```bash
$ rclone moveto level3:mydir level3:mydir2  # mydir on even+odd, missing on parity
# ❌ Fails with error
# ERROR: "parity dirmove failed: rename .../mydir .../mydir2: no such file or directory"
# Source remains unchanged
```

**Implementation**: Check `auto_heal` before `Mkdir()` fallback in `DirMove()`

---

## Code Changes

### 1. Configuration

```go
// In init() - Register option
{
    Name:     "auto_heal",
    Help:     "Automatically reconstruct missing particles/directories (2/3 present)",
    Default:  true,
    Advanced: false,
}

// In Options struct
type Options struct {
    // ... existing fields ...
    AutoCleanup bool   `config:"auto_cleanup"`
    AutoHeal    bool   `config:"auto_heal"`  // NEW
}

// In NewFs() - Apply default
if _, ok := m.Get("auto_heal"); !ok {
    opt.AutoHeal = true
}
```

### 2. List() - Directory Reconstruction

```go
// At end of List()
if f.opt.AutoHeal {
    f.reconstructMissingDirectory(ctx, dir, errEven, errOdd)
}
```

### 3. Open() - File Particle Self-Healing

```go
// In Open() after reconstruction
if o.fs.opt.AutoHeal {
    _, oddData := SplitBytes(merged)
    o.fs.queueParticleUpload(o.remote, "odd", oddData, isOddLength)
}
```

### 4. DirMove() - Best-Effort Reconstruction

```go
// In each backend move goroutine
if (os.IsNotExist(err) || errors.Is(err, fs.ErrorDirNotFound)) && f.opt.AutoHeal {
    fs.Infof(f, "DirMove: source missing on %s, creating destination (reconstruction)", backendName)
    return f.backend.Mkdir(gCtx, "")
}
```

---

## Tests Added

### TestAutoHealDirectoryReconstruction
Tests directory reconstruction during `List()`:
- ✅ auto_heal=true: Creates missing directory
- ✅ auto_heal=false: Does NOT create missing directory

### TestAutoHealDirMove  
Tests directory reconstruction during `DirMove()`:
- ✅ auto_heal=true: Best-effort move with reconstruction
- ✅ auto_heal=false: Fails if directory degraded

### TestDirMove (updated)
Tests normal directory renaming when all 3 backends have directory

---

## User Experience Examples

### Production Use (Both Enabled)
```bash
# rclone.conf
[myremote]
type = level3
even = s3even:
odd = s3odd:
parity = s3parity:
auto_cleanup = true  # Hide orphans (1/3)
auto_heal = true     # Reconstruct degraded (2/3)
```

**Result**: Fully automatic - hides broken items, reconstructs degraded items

---

### Conservative Mode (Cleanup Only)
```bash
# rclone.conf
[myremote]
type = level3
even = s3even:
odd = s3odd:
parity = s3parity:
auto_cleanup = true   # Hide orphans
auto_heal = false     # NO auto-reconstruction
```

**Result**: Clean listings, but no automatic healing (must rebuild manually)

**Use Case**: 
- High-performance scenarios (avoid background uploads)
- Manual control over healing process
- Scheduled bulk rebuilds instead of opportunistic healing

---

### Debugging Mode (See Everything)
```bash
# rclone.conf
[myremote]
type = level3
even = s3even:
odd = s3odd:
parity = s3parity:
auto_cleanup = false  # Show orphans
auto_heal = false     # No healing
```

**Result**: Raw view of all particles, no automatic changes

**Use Case**:
- Investigating backend failures
- Understanding what particles are missing
- Manual particle management
- Testing/development

---

## Performance Considerations

### Why Separate Flags?

**auto_cleanup** (lightweight):
- Just hides items from listings (filter operation)
- Deletes orphaned particles when accessed
- Minimal performance impact

**auto_heal** (moderate cost):
- Creates directories (API calls)
- Queues particle uploads (background I/O)
- Can be disabled for performance

**Performance Impact**:

| Operation | auto_heal=true | auto_heal=false |
|-----------|----------------|-----------------|
| List empty degraded dir | +1 Mkdir API call | No extra calls |
| List 100 degraded dirs | +100 Mkdir calls | No extra calls |
| Read degraded file | +1 background upload | No extra uploads |
| Read 100 degraded files | +100 background uploads | No extra uploads |

**Use auto_heal=false when**:
- Batch processing large datasets with known degradation
- Want to defer healing to scheduled maintenance window
- Need fastest possible list operations
- Investigating degraded state without changing it

---

## Implementation Notes

### Directory Reconstruction is Immediate
Unlike file particle reconstruction (background queue), directory reconstruction happens immediately during `List()`:
- Directories are lightweight (just metadata)
- Creating directory is fast (one API call)
- No data transfer involved
- Safe operation (idempotent)

### File Reconstruction is Background
File particle uploads are queued and happen in background:
- Prevents blocking read operations
- Uses background worker pool
- Gradual, opportunistic healing
- Can be disabled with auto_heal=false

---

## Testing

All tests pass:
```
✅ TestAutoHealDirectoryReconstruction
   ✅ auto_heal=true reconstructs missing directory
   ✅ auto_heal=false does NOT reconstruct missing directory
   
✅ TestAutoHealDirMove
   ✅ auto_heal=true reconstructs during DirMove
   ✅ auto_heal=false fails DirMove with degraded directory
   
✅ TestDirMove
   ✅ Directory renaming works when all backends healthy
```

---

## Documentation Updated

✅ `README.md`:
- Added auto_heal section
- Behavior matrix with both flags
- Examples for each mode
- State definitions (degraded vs orphaned)

✅ `RAID3_SEMANTICS_DISCUSSION.md`:
- Comprehensive behavior definition
- Terminology agreement
- Decision rationale

---

## Summary

**New Feature**: `auto_heal` flag for controlling automatic reconstruction

**Behavior**:
- **2/3 particles (degraded)**: Reconstruct if auto_heal=true
- **1/3 particles (orphaned)**: Hide/delete if auto_cleanup=true

**Benefits**:
- ✅ Separate concerns (cleanup vs healing)
- ✅ Performance control (disable healing for speed)
- ✅ Debugging flexibility (see raw state)
- ✅ Consistent RAID3 semantics

**Default**: Both enabled (auto_cleanup=true, auto_heal=true) for best user experience

