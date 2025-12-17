# Investigation Summary: List("") Behavior and Union Backend Pattern

## Executive Summary

Investigated why `Rmdir` fix is failing and identified operations that should follow union backend's "check first" pattern.

## Key Findings

### 1. List("") Behavior Issue

**Problem**: `Rmdir` check for directory existence using `List("")` is not working as expected.

**Test Setup**:
- Test creates `TestRAID3:rclone-test-xxxxx` (subdirectory path)
- Underlying backends: `evenDir/rclone-test-xxxxx`, `oddDir/rclone-test-xxxxx`, `parityDir/rclone-test-xxxxx`
- Subdirectory `rclone-test-xxxxx` **doesn't exist** (hasn't been created via `Mkdir()`)

**Expected Behavior**:
- `f.even.List(ctx, "")` should return `(nil, fs.ErrorDirNotFound)` (subdirectory doesn't exist)
- Our check should detect all three return `ErrorDirNotFound`
- Should return `fs.ErrorDirNotFound` immediately

**Actual Behavior**:
- Test fails, meaning `List("")` is not returning `ErrorDirNotFound` as expected
- Possible causes:
  1. Health check creates the directory (unlikely)
  2. Auto-heal creates it (unlikely - needs 2/3 to exist)
  3. Test environment has leftover state
  4. `List("")` behavior differs from expected

### 2. Union Backend "Check First" Pattern

**Operations that check existence BEFORE action** (using `action` policy):
1. **Rmdir** - Checks if directory exists before removing
2. **Purge** - Checks if root exists before purging
3. **DirMove** - Checks if source directory exists before moving
4. **DirSetModTime** - Checks if directory exists before setting modtime
5. **Move** - Checks if source object exists before moving
6. **Remove** - Checks if object exists before removing
7. **Update** - Checks if object exists before updating
8. **SetModTime** - Checks if object exists before setting modtime

**How it works**:
- `action` policy uses `findEntry()` which calls `List()` to check existence
- For root directory: If `List("")` returns error → doesn't exist → return `ErrorObjectNotFound`
- If `List("")` returns no error (even empty list) → exists → proceed with operation

### 3. Operations We Should Consider

**Already Implemented (Health Check Pattern)**:
- ✅ Put, Update, Move, Mkdir, SetModTime, DirMove - Use `checkAllBackendsAvailable` (strict write policy for RAID3)

**Should Follow Union Pattern**:
- ❓ **Purge** - Union backend checks existence first (`f.action(ctx, "")`)
- ❓ **DirSetModTime** - Union backend checks existence first (`f.action(ctx, dir)`)

**Different Pattern (May Be Acceptable)**:
- ❓ **Remove** - Union backend checks existence first, but our idempotent delete (ignore "not found") might be acceptable for RAID3's use case

## Recommendations

1. **Debug List("") behavior** - Add logging to see what it actually returns in the test
2. **Verify test environment** - Ensure clean state before test runs
3. **Fix Rmdir** - Once List behavior is understood, fix the existence check
4. **Consider Purge and DirSetModTime** - Evaluate if they should check existence first like union backend
5. **Document decision on Remove** - Decide if idempotent delete is acceptable or if we should check existence first

## Files Modified

- `backend/raid3/raid3.go` - Updated `Rmdir` to follow union backend pattern (needs further investigation)
- `backend/raid3/_analysis/LIST_AND_UNION_PATTERN_INVESTIGATION.md` - Detailed investigation document
- `backend/raid3/_analysis/RMDIR_NOT_FOUND_INVESTIGATION.md` - Original investigation document

## Status

- ✅ **Error handling pattern fixed** - Now matches union backend (return error if ANY backend returns error)
- ❓ **Root directory existence check** - Needs further investigation to understand why `List("")` isn't detecting non-existent directory
- ❓ **Other operations** - Purge and DirSetModTime should be evaluated for "check first" pattern
