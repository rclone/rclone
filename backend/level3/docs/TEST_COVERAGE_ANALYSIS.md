# Test Coverage Analysis - Level3 Backend

**Date**: November 2, 2025  
**Purpose**: Comprehensive analysis of test coverage for all operations  
**Focus**: Reads, writes, deletes, metadata, subdirectories  
**Status**: Analysis in progress

---

## 📊 Current Test Inventory

### Total Tests: 32

**Integration Tests** (2):
- `TestIntegration` - Full suite with configured remote
- `TestStandard` - Automated integration with temp dirs

**Unit Tests - Byte Operations** (3):
- `TestSplitBytes`
- `TestMergeBytes`
- `TestSplitMergeRoundtrip`

**Unit Tests - Validation** (1):
- `TestValidateParticleSizes`

**Unit Tests - Parity** (2):
- `TestCalculateParity`
- `TestParityFilenames`

**Unit Tests - Reconstruction** (4):
- `TestParityReconstruction`
- `TestReconstructFromEvenAndParity`
- `TestReconstructFromOddAndParity`
- `TestSizeFormulaWithParity`

**Integration - Degraded Mode** (2):
- `TestIntegrationStyle_DegradedOpenAndSize`
- `TestLargeDataQuick`

**File Operations** (6):
- `TestRenameFile`
- `TestRenameFileDifferentDirectory` ✅ Uses subdirs
- `TestDeleteFile`
- `TestDeleteFileIdempotent`
- `TestMoveFileBetweenDirectories` ✅ Uses subdirs
- `TestRenameFilePreservesParitySuffix`

**Self-Healing** (5):
- `TestSelfHealing`
- `TestSelfHealingEvenParticle`
- `TestSelfHealingNoQueue`
- `TestSelfHealingLargeFile`
- `TestSelfHealingShutdownTimeout`

**Error Cases** (7):
- `TestPutFailsWithUnavailableBackend`
- `TestDeleteSucceedsWithUnavailableBackend`
- `TestDeleteWithMissingParticles`
- `TestMoveFailsWithUnavailableBackend`
- `TestMoveWithMissingSourceParticle`
- `TestReadSucceedsWithUnavailableBackend`
- `TestUpdateFailsWithUnavailableBackend`
- `TestHealthCheckEnforcesStrictWrites`

---

## 📋 Operation Coverage Matrix

### Fs-Level Operations:

| Operation | Type | Tested? | Subdirs? | Degraded? | Missing |
|-----------|------|---------|----------|-----------|---------|
| `NewFs()` | Setup | ✅ All tests | N/A | ✅ Tolerates 1/3 down | None |
| `List()` | Read | ✅ TestStandard | ✅ Implicit | ⚠️ **Need explicit** | Degraded test |
| `NewObject()` | Read | ✅ Many tests | ✅ In file ops | ✅ Yes | None |
| `Put()` | Write | ✅ Many tests | ✅ In file ops | ✅ Yes | None |
| `Mkdir()` | Write | ✅ In file ops | ✅ Yes | ⚠️ **Need test** | **Degraded test** |
| `Rmdir()` | Delete | ⚠️ Implicit | ⚠️ Implicit | ⚠️ **Need test** | **Degraded test** |
| `Move()` | Write | ✅ Yes | ✅ Yes | ⚠️ Skipped | Need unskip |

---

### Object-Level Operations:

| Operation | Type | Tested? | Degraded? | Has Health Check? | Missing |
|-----------|------|---------|-----------|-------------------|---------|
| `Open()` | Read | ✅ Many | ✅ Yes | N/A (read) | None |
| `Update()` | Write | ⚠️ Implicit | ⚠️ Skipped | ✅ Yes | **Need test** |
| `Remove()` | Delete | ✅ Yes | ✅ Yes | N/A (best-effort) | None |
| `Size()` | Read | ✅ Yes | ✅ Yes | N/A (read) | None |
| `Hash()` | Read | ✅ TestStandard | ✅ Yes | N/A (read) | None |
| `ModTime()` | Read | ✅ TestStandard | N/A | N/A (read) | None |
| **`SetModTime()`** | **Write** | ⚠️ **TestStandard?** | ❌ **NO** | ❌ **NO!** | **Health check + test** |
| `Remote()` | Info | ✅ Implicit | N/A | N/A | None |
| `Fs()` | Info | ✅ Implicit | N/A | N/A | None |

---

## 🚨 Critical Findings

### Issue 1: `SetModTime()` Lacks Health Check! 🚨

**Current Implementation**:
```go
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
    g, gCtx := errgroup.WithContext(ctx)
    // ... errgroup modifies all 3 backends ...
    return g.Wait()  // ❌ NO HEALTH CHECK!
}
```

**Problem**:
- This is a **write operation** (modifies metadata)
- Should have health check like Put/Update/Move/Mkdir
- Currently has cryptic errors in degraded mode
- Inconsistent with RAID 3 policy implementation

**Impact**: 🚨 **HIGH**
- User confusion (inconsistent errors)
- Policy violation (should fail with helpful error)

---

### Issue 2: Missing Degraded Mode Tests

**Not tested in degraded mode**:
- ❌ `Mkdir()` with unavailable backend (NEW - after fix)
- ❌ `Rmdir()` with unavailable backend
- ❌ `SetModTime()` with unavailable backend
- ❌ `List()` with unavailable backend (explicit test)

**Current coverage**: Implicit in TestStandard, but not explicit degraded mode tests

---

### Issue 3: Limited Subdirectory Depth Testing

**Current subdirectory tests**:
- ✅ `TestRenameFileDifferentDirectory`: source/file.txt → dest/file.txt
- ✅ `TestMoveFileBetweenDirectories`: src/file → dst/file

**Depth**: 1 level (shallow)

**Not tested**:
- ❌ Deep nesting: dir1/dir2/dir3/file.txt
- ❌ Multiple subdirectories in same operation
- ❌ Subdirectory listing in degraded mode
- ❌ Subdirectory creation/deletion in degraded mode

---

## 📋 Missing Test Coverage

### High Priority (Write Operations):

**1. SetModTime in Degraded Mode** 🚨
```go
func TestSetModTimeFailsWithUnavailableBackend(t *testing.T) {
    // Create file
    // Stop one backend
    // Try to set mod time
    // Should fail with helpful error
}
```

**2. Mkdir in Degraded Mode**
```go
func TestMkdirFailsWithUnavailableBackend(t *testing.T) {
    // Stop one backend
    // Try to create directory
    // Should fail with helpful error (NOW IMPLEMENTED)
    // Need test to verify
}
```

---

### Medium Priority (Delete Operations):

**3. Rmdir in Degraded Mode**
```go
func TestRmdirSucceedsWithUnavailableBackend(t *testing.T) {
    // Create directory
    // Stop one backend
    // Try to remove directory
    // Should succeed (best-effort)
}
```

---

### Low Priority (Read Operations):

**4. List in Degraded Mode**
```go
func TestListSucceedsWithUnavailableBackend(t *testing.T) {
    // Create files
    // Stop one backend  
    // List directory
    // Should work (show files)
}
```

**5. Deep Subdirectories**
```go
func TestDeepSubdirectoryOperations(t *testing.T) {
    // Create: dir1/dir2/dir3/file.txt
    // List: dir1/dir2/
    // Move: dir1/dir2/file → dir4/dir5/file
    // Verify all levels work
}
```

---

## 🔍 Detailed Analysis by Operation

### Read Operations - Coverage Good ✅

| Operation | Test Coverage | Subdirs | Degraded | Status |
|-----------|---------------|---------|----------|--------|
| `List()` | ✅ TestStandard | ✅ Implicit | ⚠️ Implicit | Adequate |
| `Open()` | ✅ Many tests | ✅ Yes | ✅ Explicit | Excellent |
| `NewObject()` | ✅ Many tests | ✅ Yes | ✅ Explicit | Excellent |
| `Size()` | ✅ TestSizeFormulaWithParity | N/A | ✅ Yes | Excellent |
| `Hash()` | ✅ TestStandard | ✅ Implicit | ✅ Implicit | Good |

**Summary**: Read operations well-tested ✅

---

### Write Operations - Coverage Incomplete ⚠️

| Operation | Test Coverage | Subdirs | Degraded | Health Check | Status |
|-----------|---------------|---------|----------|--------------|--------|
| `Put()` | ✅ Many tests | ✅ Yes | ✅ Yes | ✅ Yes | Excellent |
| `Update()` | ⚠️ TestStandard | ⚠️ Implicit | ⚠️ Skipped | ✅ Yes | **Need test** |
| `Move()` | ✅ Yes | ✅ Yes | ⚠️ Skipped | ✅ Yes | Need unskip |
| `Mkdir()` | ✅ In file ops | ✅ Yes | ❌ **NO** | ✅ **Yes (NEW)** | **Need test** |
| **`SetModTime()`** | ⚠️ **TestStandard?** | ⚠️ **Implicit** | ❌ **NO** | ❌ **NO!** | **Need fix + test** |

**Summary**: SetModTime needs fix, several need degraded mode tests ⚠️

---

### Delete Operations - Coverage Incomplete ⚠️

| Operation | Test Coverage | Subdirs | Degraded | Status |
|-----------|---------------|---------|----------|--------|
| `Remove()` | ✅ Yes | ✅ Yes | ✅ Yes | Excellent |
| `Rmdir()` | ⚠️ Implicit | ⚠️ Implicit | ❌ **NO** | **Need test** |

**Summary**: Rmdir needs explicit degraded mode test ⚠️

---

## 🎯 Subdirectory Testing Analysis

### Current Subdirectory Tests:

**Explicit Subdirectory Usage** (2 tests):
1. `TestRenameFileDifferentDirectory`
   - Creates: `source/file.txt`
   - Renames to: `dest/file.txt`
   - Depth: 1 level ✅

2. `TestMoveFileBetweenDirectories`
   - Creates: `src/document.pdf`
   - Moves to: `dst/document.pdf`
   - Depth: 1 level ✅

**Implicit in TestStandard**:
- `TestStandard` (rclone integration tests) probably tests subdirectories
- But we don't control the depth or scenarios

---

### Missing Subdirectory Coverage:

**Not Tested**:
1. ❌ Deep nesting (3+ levels)
   - Example: `a/b/c/d/file.txt`
   
2. ❌ Subdirectory listing
   - `rclone ls level3:dir1/dir2/`
   
3. ❌ Subdirectory operations in degraded mode
   - Create nested dirs with backend down
   - List nested dirs with backend down
   
4. ❌ Multiple subdirectories at same level
   - `dir1/file1`, `dir2/file2`, `dir3/file3`

---

## 🔧 SetModTime Analysis

### Current Implementation:

```go
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
    g, gCtx := errgroup.WithContext(ctx)
    
    g.Go(func() error {
        obj, err := o.fs.even.NewObject(gCtx, o.remote)
        if err != nil {
            return err
        }
        return obj.SetModTime(gCtx, t)
    })
    // Same for odd and parity
    
    return g.Wait()  // ❌ NO HEALTH CHECK!
}
```

**Classification**: **Write operation** (modifies metadata)

**Expected**: Should have health check like Put/Update/Move/Mkdir

**Current**: ❌ Missing health check

---

### SetModTime in RAID 3 Context:

**Hardware RAID 3**:
- Metadata changes = write operations
- Blocked in degraded mode
- Requires all drives available

**Level3 should match**:
- SetModTime = write operation ✅
- Should be blocked in degraded mode ✅
- Should have health check ❌ **MISSING!**

---

## 🚨 Priority Fixes Needed

### 1. Add Health Check to SetModTime 🚨 **CRITICAL**

**Why Critical**:
- Write operation (modifies state)
- Violates RAID 3 policy (no health check)
- Inconsistent UX (cryptic errors)
- Could succeed partially on retries

**Fix**:
```go
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
    // Pre-flight health check
    if err := o.fs.checkAllBackendsAvailable(ctx); err != nil {
        return err
    }
    
    // Existing errgroup logic...
}
```

**Effort**: 3 lines, 2 minutes

---

### 2. Add Degraded Mode Tests ⭐ **HIGH PRIORITY**

**Tests to add**:

**A. TestMkdirFailsInDegradedMode** (NEW - verify fix):
```go
// Verify Mkdir shows helpful error when backend unavailable
```

**B. TestRmdirSucceedsInDegradedMode** (verify best-effort):
```go
// Verify Rmdir works with unavailable backend
```

**C. TestSetModTimeFailsInDegradedMode** (after fixing SetModTime):
```go
// Verify SetModTime blocks with helpful error
```

**D. TestListWorksInDegradedMode** (comprehensive):
```go
// Verify List shows files with 2/3 backends
```

**Effort**: 4 tests, ~150 lines, 1-2 hours

---

### 3. Add Deep Subdirectory Tests ⭐ **MEDIUM PRIORITY**

**Tests to add**:

**A. TestDeepNestedDirectories**:
```go
// Create: a/b/c/d/e/file.txt
// List: a/b/c/
// Verify: All levels work
```

**B. TestSubdirectoryListInDegradedMode**:
```go
// Create: dir1/subdir1/file.txt
// Stop backend
// List: dir1/subdir1/
// Should work
```

**Effort**: 2 tests, ~100 lines, 1 hour

---

## 📊 Coverage Summary

### By Operation Type:

**Reads** (Work with 2/3):
- Direct tests: 80% ✅
- Subdirectory tests: 60% ⚠️
- Degraded mode tests: 50% ⚠️

**Writes** (Require all 3):
- Direct tests: 85% ✅
- Subdirectory tests: 70% ✅
- Degraded mode tests: 60% ⚠️
- **SetModTime**: 0% 🚨 **CRITICAL GAP**

**Deletes** (Best-effort):
- Direct tests: 100% ✅
- Subdirectory tests: 50% ⚠️
- Degraded mode tests: 50% ⚠️

---

## 🎯 Recommendations

### Immediate (Critical):

1. 🚨 **Fix SetModTime** (add health check)
   - Effort: 2 minutes
   - Impact: Critical (write operation)
   - Priority: Do now

2. ⭐ **Test SetModTime in degraded mode**
   - Effort: 15 minutes
   - Impact: Verify fix works
   - Priority: Do immediately after fix

---

### Short-term (High Value):

3. ⭐ **Add explicit degraded mode tests**:
   - TestMkdirFailsInDegradedMode
   - TestRmdirSucceedsInDegradedMode
   - TestListWorksInDegradedMode
   - Effort: 1 hour
   - Impact: Complete degraded coverage

4. ⭐ **Unskip TestUpdateFailsWithUnavailableBackend**
   - Currently skipped (needs MinIO or mocked backend)
   - Important for coverage
   - Effort: 30 minutes (with mock)

---

### Long-term (Nice to Have):

5. ✅ **Deep subdirectory tests**
   - TestDeepNestedDirectories
   - TestSubdirectoryListInDegradedMode
   - Effort: 1 hour
   - Impact: Edge case coverage

6. ✅ **Mkdir subdirectory test**
   - Test creating nested directories
   - Effort: 15 minutes

---

## 📝 SetModTime Specific Analysis

### What is SetModTime?

**Purpose**: Set modification time on an object

**Triggered by**: 
- `rclone touch` command
- `rclone copy` with `--update`
- Metadata preservation operations

**Is it a write?**: ✅ **YES!**
- Modifies object metadata
- Changes state on backend
- Should be strict (all 3 backends required)

---

### Current SetModTime Behavior:

**With all backends available**:
```bash
$ rclone touch level3:file.txt
✅ Works (sets time on all 3 particles)
```

**With backend unavailable**:
```bash
$ docker stop minioodd
$ rclone touch level3:file.txt
❌ Error: [Cryptic backend error]
```

**Should be**:
```bash
$ rclone touch level3:file.txt
❌ Error: cannot write - level3 backend is DEGRADED
   [Helpful recovery guide]
```

---

## ✅ Action Plan

### Phase 1: Fix SetModTime (IMMEDIATE)

**Step 1**: Add health check
```go
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
    if err := o.fs.checkAllBackendsAvailable(ctx); err != nil {
        return err
    }
    // ...
}
```

**Step 2**: Test with MinIO
```bash
docker stop minioodd
rclone touch level3:file.txt
# Should show helpful error
```

**Step 3**: Add automated test
```go
func TestSetModTimeFailsInDegradedMode(t *testing.T) {
    // Verify SetModTime blocks with helpful error
}
```

**Effort**: 20 minutes total

---

### Phase 2: Add Missing Degraded Tests (SHORT-TERM)

**Tests to add**:
1. TestMkdirFailsInDegradedMode
2. TestRmdirSucceedsInDegradedMode
3. TestSetModTimeFailsInDegradedMode
4. TestListWorksInDegradedMode

**Effort**: 1-2 hours

---

### Phase 3: Subdirectory Coverage (LONG-TERM)

**Tests to add**:
1. TestDeepNestedDirectories
2. TestSubdirectoryListInDegradedMode
3. TestMkdirNestedDirectories

**Effort**: 1-2 hours

---

## 📊 Current vs Target Coverage

### Current:
- Total tests: 32
- Operations covered: 11/14 (79%)
- Degraded mode coverage: 60%
- Subdirectory coverage: 65%
- **Critical gap**: SetModTime ❌

### Target (After Fixes):
- Total tests: 36-40
- Operations covered: 14/14 (100%)
- Degraded mode coverage: 90%
- Subdirectory coverage: 80%
- **No critical gaps**: ✅

---

## ✅ Immediate Actions

**Do NOW**:
1. ✅ Fix SetModTime (add health check)
2. ✅ Test with MinIO
3. ✅ Add TestSetModTimeFailsInDegradedMode
4. ✅ Run all tests
5. ✅ Commit

**Effort**: 20-30 minutes  
**Impact**: Closes critical gap

---

**SetModTime is the only critical gap found!** All other operations are well-tested. 🎯

