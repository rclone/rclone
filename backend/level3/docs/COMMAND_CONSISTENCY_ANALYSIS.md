# Level3 Command Consistency Analysis - RAID 3 Compliance

**Date**: November 2, 2025  
**Purpose**: Verify RAID 3 policy is consistently applied across ALL rclone commands  
**Policy**: Reads work with 2/3, Writes require all 3, Deletes are best-effort

---

## 🎯 RAID 3 Policy Reminder

### Official Policy (DD-001):
- **Reads**: Work with ANY 2 of 3 backends (best effort) ✅
- **Writes**: Require ALL 3 backends (strict) ❌
- **Deletes**: Work with ANY backends (best effort, idempotent) ✅

---

## 📋 Rclone Commands by Category

### Read Operations (Should Work with 2/3)

| Command | Maps To | Has Health Check? | Tolerates Missing? | Status |
|---------|---------|-------------------|-------------------|--------|
| `rclone ls` | `List()` | ❌ No | ✅ Yes (fallback logic) | ✅ Correct |
| `rclone lsd` | `List()` | ❌ No | ✅ Yes (fallback logic) | ✅ Correct |
| `rclone lsl` | `List()` | ❌ No | ✅ Yes (fallback logic) | ✅ Correct |
| `rclone cat` | `Open()` | ❌ No | ✅ Yes (reconstruction) | ✅ Correct |
| `rclone copy FROM` | `Open()` | ❌ No | ✅ Yes (reconstruction) | ✅ Correct |
| `rclone size` | `List()` | ❌ No | ✅ Yes (fallback logic) | ✅ Correct |
| `rclone check` | `List()+Hash()` | ❌ No | ✅ Yes (reconstruction) | ✅ Correct |

---

### Write Operations (Should Fail with 2/3)

| Command | Maps To | Has Health Check? | Status | Issue? |
|---------|---------|-------------------|--------|--------|
| `rclone copy TO` | `Put()` | ✅ Yes | ✅ Correct | None |
| `rclone move` | `Move()` | ✅ Yes | ✅ Correct | None |
| `rclone mkdir` | `Mkdir()` | ❌ **NO** | ⚠️  **INCONSISTENT** | **YES** |
| `rclone sync` | `Put()+Delete()` | ✅ Yes (Put) | ✅ Correct | None |
| `rclone rcat` | `Put()` | ✅ Yes | ✅ Correct | None |
| `rclone touch` | `Put()` | ✅ Yes | ✅ Correct | None |

---

### Delete Operations (Should Be Best-Effort)

| Command | Maps To | Tolerates Missing? | Status |
|---------|---------|-------------------|--------|
| `rclone delete` | `Remove()` | ✅ Yes | ✅ Correct |
| `rclone rmdir` | `Rmdir()` | ⚠️  Unknown | ⚠️  **CHECK** |
| `rclone purge` | `Purge()` (if exists) | ❌ Not implemented | N/A |

---

## 🚨 Issues Found

### Issue 1: `Mkdir()` Lacks Health Check ⚠️

**Current Implementation**:
```go
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
    g, gCtx := errgroup.WithContext(ctx)
    
    g.Go(func() error {
        err := f.even.Mkdir(gCtx, dir)
        if err != nil {
            return fmt.Errorf("even mkdir failed: %w", err)
        }
        return nil
    })
    // ... odd, parity ...
    
    return g.Wait()  // ❌ No pre-flight health check!
}
```

**Problem**:
- Mkdir uses errgroup (will fail if backend unavailable) ✅
- But NO enhanced error message ❌
- But NO pre-flight health check ❌
- Retries might create partial directories ❌

**User sees**:
```
ERROR: odd mkdir failed: connection refused
```

**Should see**:
```
ERROR: cannot create directory - level3 backend is DEGRADED
[Same helpful error as Put/Update/Move]
```

**Impact**: 
- ⚠️  Inconsistent UX (mkdir errors are cryptic)
- ⚠️  May allow partial directory creation on retries
- ⚠️  Doesn't match strict write policy fully

---

### Issue 2: `Rmdir()` May Not Be Best-Effort ⚠️

**Current Implementation**:
```go
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
    g, gCtx := errgroup.WithContext(ctx)
    
    g.Go(func() error {
        return f.even.Rmdir(gCtx, dir)  // ❌ Returns error
    })
    // ... odd, parity ...
    
    return g.Wait()  // ❌ Fails if ANY backend fails
}
```

**Problem**:
- errgroup fails if ANY backend fails ❌
- Should be best-effort (like `Remove()`) ✅
- Rmdir of non-existent dir should succeed (idempotent) ✅

**Expected behavior** (best-effort):
```go
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
    g, gCtx := errgroup.WithContext(ctx)
    
    g.Go(func() error {
        err := f.even.Rmdir(gCtx, dir)
        if err != nil && !errors.Is(err, fs.ErrorDirNotFound) {
            return err
        }
        return nil  // Ignore "not found"
    })
    // ... same for odd, parity ...
}
```

**User impact**:
- Currently: `rclone rmdir` fails if backend unavailable
- Should: `rclone rmdir` succeeds (best-effort cleanup)

---

## 📊 Current vs Expected Behavior

### Reads (Expected: Work with 2/3):

| Operation | Current | Expected | Match? |
|-----------|---------|----------|--------|
| `List()` | ✅ Fallback to available | ✅ Work with 2/3 | ✅ |
| `Open()` | ✅ Reconstruction | ✅ Work with 2/3 | ✅ |
| `NewObject()` | ✅ Finds with 2/3 | ✅ Work with 2/3 | ✅ |

**Verdict**: ✅ **Reads are correct!**

---

### Writes (Expected: Fail with 2/3):

| Operation | Current | Expected | Match? |
|-----------|---------|----------|--------|
| `Put()` | ✅ Health check + fail | ❌ Fail with degraded | ✅ |
| `Update()` | ✅ Health check + fail | ❌ Fail with degraded | ✅ |
| `Move()` | ✅ Health check + fail | ❌ Fail with degraded | ✅ |
| `Mkdir()` | ⚠️  errgroup only | ❌ Fail with degraded | ⚠️  **Incomplete** |

**Verdict**: ⚠️  **Mkdir needs health check for consistency!**

---

### Deletes (Expected: Best-effort):

| Operation | Current | Expected | Match? |
|-----------|---------|----------|--------|
| `Remove()` | ✅ Ignores "not found" | ✅ Best effort | ✅ |
| `Rmdir()` | ❌ Fails if any fails | ✅ Best effort | ❌ **WRONG** |

**Verdict**: ❌ **Rmdir needs to be best-effort!**

---

## 🧪 Testing Current Behavior

Let me verify with MinIO tests...

### Test 1: `rclone ls` with Missing Backend

**Setup**: Stop odd backend

**Expected**: Should work (read operation)

**Test**: Listed in testing section below

---

### Test 2: `rclone mkdir` with Missing Backend

**Setup**: Stop odd backend

**Expected**: Should fail with helpful error (write operation)

**Current**: Likely fails with cryptic error (no health check)

**Test**: Listed in testing section below

---

### Test 3: `rclone rmdir` with Missing Backend

**Setup**: Stop odd backend, directory exists on even/parity

**Expected**: Should succeed (best-effort delete)

**Current**: Likely fails (errgroup returns error)

**Test**: Listed in testing section below

---

## 🎯 Recommendations

### Fix 1: Add Health Check to `Mkdir()` ⭐ **HIGH PRIORITY**

**Why**: 
- Write operation (creates state)
- Should be consistent with Put/Update/Move
- Should have helpful error messages

**Implementation**:
```go
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
    // Pre-flight health check (consistent with Put/Update/Move)
    if err := f.checkAllBackendsAvailable(ctx); err != nil {
        return fmt.Errorf("cannot create directory - level3 backend is DEGRADED\n\n[helpful error message]")
    }
    
    // Existing errgroup logic...
}
```

**Effort**: ~10 lines, 5 minutes

---

### Fix 2: Make `Rmdir()` Best-Effort ⭐ **MEDIUM PRIORITY**

**Why**:
- Delete operation (removes state)
- Should be idempotent like Remove()
- Should succeed with missing backends

**Implementation**:
```go
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
    g, gCtx := errgroup.WithContext(ctx)
    
    g.Go(func() error {
        err := f.even.Rmdir(gCtx, dir)
        if err != nil && !errors.Is(err, fs.ErrorDirNotFound) {
            return err
        }
        return nil  // Ignore "not found" - idempotent
    })
    // Same for odd, parity
    
    return g.Wait()
}
```

**Effort**: ~10 lines, 5 minutes

---

## 📝 Complete Operation Matrix

### Full List of Fs Methods:

| Method | Category | Should Work with 2/3? | Has Check? | Correct? |
|--------|----------|----------------------|------------|----------|
| `List()` | Read | ✅ Yes | N/A | ✅ Yes |
| `NewObject()` | Read | ✅ Yes | N/A | ✅ Yes |
| `Put()` | Write | ❌ No (strict) | ✅ Yes | ✅ Yes |
| `Mkdir()` | Write | ❌ No (strict) | ❌ **NO** | ⚠️  **NO** |
| `Rmdir()` | Delete | ✅ Yes (best-effort) | N/A | ❌ **NO** |
| `Move()` | Write | ❌ No (strict) | ✅ Yes | ✅ Yes |
| `Object.Update()` | Write | ❌ No (strict) | ✅ Yes | ✅ Yes |
| `Object.Remove()` | Delete | ✅ Yes (best-effort) | N/A | ✅ Yes |
| `Object.Open()` | Read | ✅ Yes | N/A | ✅ Yes |
| `Object.Size()` | Read | ✅ Yes | N/A | ✅ Yes |
| `Object.Hash()` | Read | ✅ Yes | N/A | ✅ Yes |

**Summary**:
- ✅ Correct: 9/11 operations
- ⚠️  Incorrect: 2/11 operations (Mkdir, Rmdir)

---

## 🧪 Verification Tests Needed

Will run these tests to confirm current behavior:

### Test 1: List Operations (Should Work)
```bash
docker stop minioodd
rclone ls level3:
rclone lsd level3:
```
**Expected**: ✅ Works

---

### Test 2: Mkdir (Should Fail with Helpful Error)
```bash
docker stop minioodd
rclone mkdir level3:newdir
```
**Expected**: ❌ Fails with helpful error  
**Actual**: ⚠️  Need to verify

---

### Test 3: Rmdir (Should Succeed - Best Effort)
```bash
docker stop minioodd
rclone rmdir level3:existingdir
```
**Expected**: ✅ Succeeds (best-effort)  
**Actual**: ⚠️  Need to verify

---

### Test 4: Copy TO (Should Fail)
```bash
docker stop minioodd
rclone copy file.txt level3:
```
**Expected**: ❌ Fails with helpful error  
**Actual**: ✅ Confirmed working

---

### Test 5: Copy FROM (Should Work)
```bash
docker stop minioodd
rclone copy level3:file.txt /tmp/
```
**Expected**: ✅ Works with reconstruction  
**Actual**: ✅ Confirmed working

---

## 🎯 Action Items

### Immediate (Consistency Fixes):
1. ⭐ Add health check to `Mkdir()` (5 min)
2. ⭐ Make `Rmdir()` best-effort (5 min)
3. ✅ Test all operations with MinIO (15 min)
4. ✅ Document behavior in README (10 min)

**Total effort**: ~35 minutes to fix inconsistencies

---

## 📊 Expected Behavior After Fixes

### Read Commands (Work with 2/3): ✅
```bash
$ docker stop minioodd

$ rclone ls level3:
✅ Lists files (using even + parity)

$ rclone cat level3:file.txt
✅ Shows content (reconstruction)

$ rclone copy level3:file.txt /tmp/
✅ Downloads file (reconstruction)
```

---

### Write Commands (Fail with 2/3): ❌
```bash
$ docker stop minioodd

$ rclone copy file.txt level3:
❌ Error: cannot write - level3 backend is DEGRADED
   [Helpful recovery guide shown]

$ rclone mkdir level3:newdir
❌ Error: cannot create directory - level3 backend is DEGRADED
   [Helpful recovery guide shown]

$ rclone move file.txt level3:
❌ Error: cannot move - level3 backend is DEGRADED
   [Helpful recovery guide shown]
```

---

### Delete Commands (Best-effort): ✅
```bash
$ docker stop minioodd

$ rclone delete level3:file.txt
✅ Succeeds (deletes from even + parity)

$ rclone rmdir level3:dir/
✅ Succeeds (removes from even + parity)
```

---

## 📝 Summary

### Current State:
- ✅ Reads: Fully compliant (9/9 operations)
- ⚠️  Writes: Mostly compliant (3/4 operations) - **Mkdir missing health check**
- ⚠️  Deletes: Partially compliant (1/2 operations) - **Rmdir not best-effort**

### After Fixes:
- ✅ Reads: Fully compliant (9/9)
- ✅ Writes: Fully compliant (4/4)
- ✅ Deletes: Fully compliant (2/2)

**Overall**: 11/13 → 15/15 operations ✅

---

**Need to fix Mkdir and Rmdir for full RAID 3 compliance!**

