# Investigation: List("") Behavior and Union Backend Pattern Analysis

## Problem Statement

The `Rmdir` fix is failing because `List("")` check isn't correctly detecting when the root directory doesn't exist. We need to:
1. Investigate why `List("")` behaves the way it does
2. Identify other operations that should follow union backend's "check first" pattern

## Union Backend Pattern: "Check First"

Union backend uses a **"check first"** pattern for operations that modify existing resources:

### Operations Using `action` Policy (Check Existence Before Action)

1. **Rmdir** (`union.go:127-144`):
   - Calls `f.action(ctx, dir)` to check if directory exists
   - If `action` returns `fs.ErrorObjectNotFound`, returns that error
   - Otherwise calls `Rmdir` on upstreams

2. **Purge** (`union.go:242-262`):
   - Calls `f.action(ctx, "")` to check if root exists
   - If `action` returns error, returns that error
   - Otherwise calls `Purge` on upstreams

3. **DirMove** (`union.go:395-437`):
   - Calls `sfs.action(ctx, srcRemote)` to check if source directory exists
   - If `action` returns error, returns that error
   - Otherwise performs move

4. **DirSetModTime** (`union.go:441-457`):
   - Calls `f.action(ctx, dir)` to check if directory exists
   - If `action` returns error, returns that error
   - Otherwise sets modtime

5. **Move** (`union.go:314-384`):
   - Calls `f.actionEntries(o.candidates()...)` to check if source object exists
   - If `actionEntries` returns error, returns that error
   - Otherwise performs move

6. **Remove** (`union/entry.go:110-126`):
   - Calls `o.fs.actionEntries(o.candidates()...)` to check if object exists
   - If `actionEntries` returns error, returns that error
   - Otherwise removes object

7. **Update** (`union/entry.go:69-106`):
   - Calls `o.fs.actionEntries(o.candidates()...)` to check if object exists
   - If `actionEntries` returns error, handles it (may create new object)
   - Otherwise updates object

8. **SetModTime** (`union/entry.go:130-148`):
   - Calls `o.fs.actionEntries(o.candidates()...)` to check if object exists
   - If `actionEntries` returns error, returns that error
   - Otherwise sets modtime

### Operations Using `create` Policy (Check/Create Before Action)

1. **Mkdir** (`union.go:186-188`):
   - Calls `f.mkdir(ctx, dir)` which uses `create` policy
   - Creates directory if it doesn't exist

2. **Put** (`union.go:594-609`):
   - Calls `f.put(ctx, in, src, false, options...)` which uses `create` policy
   - Creates object if it doesn't exist

### How `action` Policy Works

The `action` policy (e.g., `EpAll`) uses `findEntry()` to check if a path exists:

```go
func findEntry(ctx context.Context, f fs.Fs, remote string) fs.DirEntry {
    remote = clean(remote)
    dir := parentDir(remote)
    entries, err := f.List(ctx, dir)
    if remote == dir {
        // For root directory (remote == dir == "")
        if err != nil {
            return nil  // Directory doesn't exist
        }
        return fs.NewDir("", time.Time{})  // Directory exists
    }
    // For subdirectories, search in entries
    // ...
}
```

**Key Insight**: For root directory (`remote == dir == ""`):
- If `List("")` returns an **error**, directory doesn't exist → `findEntry` returns `nil`
- If `List("")` returns **no error** (even if empty list), directory exists → `findEntry` returns directory entry

## Our List("") Implementation Issue

**Our `List` function** (`raid3.go:844-1012`):

```go
// If even fails, try odd
if errEven != nil {
    if errOdd != nil {
        // Both data backends failed
        if f.opt.AutoCleanup {
            // Check parity - if it exists, remove orphaned directory
            _, errParity := f.parity.List(ctx, dir)
            if errParity == nil {
                // Parity exists - remove orphaned directory
                _ = f.parity.Rmdir(ctx, dir)
                return nil, nil // Return empty list, no error
            }
        }
        return nil, errEven // Return even error
    }
    // Continue with odd entries
}
```

**Problem**: When root doesn't exist:
1. `even.List("")` returns `fs.ErrorDirNotFound`
2. `odd.List("")` returns `fs.ErrorDirNotFound`
3. `parity.List("")` might return `fs.ErrorDirNotFound` or `nil` (empty list)
4. Our logic might return `nil, nil` (empty list) instead of `fs.ErrorDirNotFound`

**Root Cause**: Our `List` function has special logic for degraded mode (works with 2/3 backends) and auto-cleanup that might mask the "directory doesn't exist" case.

## Investigation: What Does List("") Actually Return?

### Expected Behavior (Local Backend)
- When root doesn't exist: `List("")` should return `(nil, fs.ErrorDirNotFound)`
- When root exists (empty): `List("")` should return `([]fs.DirEntry{}, nil)`

### Our Composite List Behavior
- When all 3 backends return `ErrorDirNotFound`: Should return `(nil, fs.ErrorDirNotFound)`
- But our logic might be interfering with auto-cleanup or degraded mode handling

## Operations That Should Follow Union Pattern

Based on union backend analysis, these operations should check existence first:

### Already Implemented (Have Health Check)
- ✅ **Put** - Uses `checkAllBackendsAvailable` (strict write policy)
- ✅ **Update** - Uses `checkAllBackendsAvailable` (strict write policy)
- ✅ **Move** - Uses `checkAllBackendsAvailable` (strict write policy)
- ✅ **Mkdir** - Uses `checkAllBackendsAvailable` (strict write policy)
- ✅ **SetModTime** - Uses `checkAllBackendsAvailable` (strict write policy)
- ✅ **DirMove** - Uses `checkAllBackendsAvailable` (strict write policy)

### Need Investigation
- ❓ **Rmdir** - Currently checking with `List("")` but failing
- ❓ **Purge** - Should check existence first (union backend does)
- ❓ **Remove** - Uses `errgroup`, but should check existence first (union backend does)
- ❓ **CleanUp** - Should check existence first?

### Notes
- **Remove**: Union backend checks existence via `actionEntries` before removing. Our implementation uses `errgroup` and ignores "not found" errors (idempotent delete), which is different but might be acceptable for RAID3.
- **Purge**: Union backend checks existence via `action("")` before purging. Our implementation should probably do the same.

## Investigation Findings

### Issue with Current Rmdir Implementation

**Current Code** (lines 1240-1266):
```go
dirExists := false
_, evenListErr := f.even.List(ctx, dir)
if evenListErr == nil {
    dirExists = true
} else if !errors.Is(evenListErr, fs.ErrorDirNotFound) {
    dirExists = true  // Non-ErrorDirNotFound error - assume exists
}
// ... same for odd and parity
if !dirExists {
    return fs.ErrorDirNotFound
}
```

**Problem**: The logic looks correct, but the test is still failing. This suggests:
1. `List("")` might be returning something unexpected
2. The root directory might actually exist (created by test setup or health check)
3. There might be a race condition or timing issue

### Potential Issues

1. **Health Check Side Effects**: `checkAllBackendsAvailable()` calls `List("")` on each backend (line 661). This shouldn't create directories, but we should verify.

2. **Auto-Heal Side Effects**: `reconstructMissingDirectory()` (line 1783) creates missing directories when 2/3 backends have them. If this runs during `List()`, it might create the root.

3. **Auto-Cleanup Side Effects**: `cleanupOrphanedDirectory()` (line 1839) removes orphaned directories. This shouldn't create directories, but might affect state.

4. **Test Setup**: The test calls `Rmdir("")` BEFORE `Mkdir("")`, so root shouldn't exist. But maybe test setup or previous tests create it?

### What We Need to Verify

1. **What does `f.even.List(ctx, "")` actually return** when root doesn't exist?
   - Expected: `(nil, fs.ErrorDirNotFound)`
   - Actual: Need to test

2. **Does health check create root?**
   - Health check calls `List("")` but shouldn't create directories
   - Need to verify

3. **Does auto-heal create root during List?**
   - `reconstructMissingDirectory` is called from `List()` (line 1009)
   - If 2/3 backends have root, it creates it on the third
   - This might be the issue!

4. **Test environment state**
   - Does the test run in a clean state?
   - Are there leftover directories from previous tests?

## Root Cause Analysis

### Test Setup

**Test creates**: `TestRAID3:rclone-test-xxxxx` (via `RandomRemoteName`)
- `subRemoteName` = `"TestRAID3:rclone-test-xxxxx"`
- Underlying backends point to temp directories: `evenDir`, `oddDir`, `parityDir`
- Each backend's root is `rclone-test-xxxxx` (a subdirectory inside the temp dirs)

**When `Rmdir("")` is called**:
- We're trying to remove the root directory `rclone-test-xxxxx` from each backend
- This subdirectory **doesn't exist** (hasn't been created via `Mkdir()`)
- So `f.even.List(ctx, "")` should list `evenDir/rclone-test-xxxxx`, which doesn't exist
- Should return `(nil, fs.ErrorDirNotFound)`

### Expected Behavior

**Local backend `List("")` on non-existent directory**:
- `os.Stat("evenDir/rclone-test-xxxxx")` fails with `os.ErrNotExist`
- Returns `(nil, fs.ErrorDirNotFound)` ✅

**Our check**:
- `evenListErr == fs.ErrorDirNotFound` → `dirExists = false`
- Same for odd and parity
- All three return `ErrorDirNotFound` → `dirExists = false`
- Should return `fs.ErrorDirNotFound` ✅

### Why It's Failing

The test is still failing, which means one of:
1. **`List("")` is not returning `ErrorDirNotFound`** - Maybe the subdirectory exists?
2. **Health check creates the subdirectory** - `checkAllBackendsAvailable()` might create it
3. **Auto-heal creates the subdirectory** - If 2/3 backends have it, auto-heal creates it on the third
4. **Test setup creates the subdirectory** - Maybe `NewFs` or test setup creates it

### Most Likely Culprit: Health Check

**`checkAllBackendsAvailable()` behavior** (line 661-672):
- Calls `backend.List(checkCtx, "")` on each backend
- If `List("")` returns `ErrorDirNotFound`, that's acceptable (line 668)
- Then calls `backend.Mkdir(checkCtx, testDir)` where `testDir = ".raid3-health-check-" + name`
- **But**: If the root directory doesn't exist, `Mkdir` might create parent directories?

Actually, `Mkdir` on a test subdirectory shouldn't create the root. But let me check if there's something else.

### Other Potential Issues

1. **Auto-heal during List**: If `List("")` is called and 2/3 backends have the directory, `reconstructMissingDirectory` creates it on the third (line 1009). But if none have it, this shouldn't trigger.

2. **Test environment**: Maybe the subdirectory exists from a previous test run?

3. **NewFs behavior**: Does `NewFs` create the root directory? Let me check.

## Investigation Summary

### Key Findings

1. **Test Setup**: 
   - Creates `TestRAID3:rclone-test-xxxxx` via `RandomRemoteName`
   - Underlying backends point to temp directories with root `rclone-test-xxxxx`
   - This subdirectory **doesn't exist** (hasn't been created via `Mkdir()`)

2. **Expected Behavior**:
   - `f.even.List(ctx, "")` should list `evenDir/rclone-test-xxxxx` (doesn't exist)
   - Should return `(nil, fs.ErrorDirNotFound)`
   - Our check should see all three return `ErrorDirNotFound`
   - Should return `fs.ErrorDirNotFound` ✅

3. **Why It's Failing**:
   - Test still fails, meaning `List("")` is not returning `ErrorDirNotFound`
   - Possible causes:
     - Health check creates the directory (unlikely - only creates test subdirectory)
     - Auto-heal creates it (unlikely - needs 2/3 to exist first)
     - Test environment has leftover directories
     - `List("")` behavior is different than expected

4. **Union Backend Operations Using "Check First" Pattern**:
   - ✅ **Rmdir** - Uses `action` policy (check existence first)
   - ✅ **Purge** - Uses `action("")` policy (check root existence first)
   - ✅ **DirMove** - Uses `action(srcRemote)` policy (check source existence first)
   - ✅ **DirSetModTime** - Uses `action(dir)` policy (check existence first)
   - ✅ **Move** - Uses `actionEntries` (check source object existence first)
   - ✅ **Remove** - Uses `actionEntries` (check object existence first)
   - ✅ **Update** - Uses `actionEntries` (check object existence first)
   - ✅ **SetModTime** - Uses `actionEntries` (check object existence first)

### Operations We Should Consider

**Already Have Health Check (Different Pattern, But Correct for RAID3)**:
- ✅ Put, Update, Move, Mkdir, SetModTime, DirMove - Use `checkAllBackendsAvailable` (strict write policy)

**Should Follow Union Pattern (Check Existence First)**:
- ❓ **Purge** - Union backend checks existence first (line 248: `f.action(ctx, "")`)
- ❓ **DirSetModTime** - Union backend checks existence first (line 442: `f.action(ctx, dir)`)

**Different Pattern (May Be Acceptable)**:
- ❓ **Remove** - Union backend checks existence first, but our idempotent delete (ignore "not found") might be acceptable for RAID3

## Next Steps

1. **Add debug logging** to see what `List("")` actually returns in the failing test
2. **Verify test environment** - Check if directories exist before test runs
3. **Check if health check creates directories** - Verify `Mkdir(testDir)` doesn't create parent
4. **Compare with union backend** - See how union backend handles this exact test scenario
5. **Consider Purge and DirSetModTime** - Should we add existence checks to these operations?

## Other Operations That Should Follow Union Pattern

Based on union backend analysis:

### Should Check Existence First (Using `action` Policy)
- ✅ **Rmdir** - Currently implementing (needs fix)
- ❓ **Purge** - Union backend checks existence first (line 248: `f.action(ctx, "")`)
- ❓ **DirSetModTime** - Union backend checks existence first (line 442: `f.action(ctx, dir)`)
- ❓ **DirMove** - Union backend checks source existence first (line 401: `sfs.action(ctx, srcRemote)`)

### Already Have Health Check (Strict Write Policy)
- ✅ **Put** - Uses `checkAllBackendsAvailable` (different from union, but correct for RAID3)
- ✅ **Update** - Uses `checkAllBackendsAvailable`
- ✅ **Move** - Uses `checkAllBackendsAvailable`
- ✅ **Mkdir** - Uses `checkAllBackendsAvailable`
- ✅ **SetModTime** - Uses `checkAllBackendsAvailable`
- ✅ **DirMove** - Uses `checkAllBackendsAvailable`

### Different Pattern (Idempotent Delete)
- ❓ **Remove** - Union backend checks existence first, but our idempotent delete (ignore "not found") might be acceptable for RAID3

### Notes
- **Remove**: Our implementation ignores "not found" errors (idempotent delete), which is different from union but might be acceptable for RAID3's use case.
- **Purge**: Should probably check existence first like union backend does.
- **DirSetModTime**: Should probably check existence first like union backend does.
