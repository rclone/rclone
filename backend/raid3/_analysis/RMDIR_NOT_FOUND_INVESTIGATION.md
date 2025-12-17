# Investigation: FsRmdirNotFound Test Failure

## Problem Statement

The test `TestStandard/FsRmdirNotFound` is failing. The test expects an error when `Rmdir("")` is called on a non-existent root directory, but our implementation returns `nil`.

## Test Expectations

**Test Location**: `fstest/fstests/fstests.go:518-526`

```go
// TestFsRmdirNotFound tests deleting a nonexistent directory
t.Run("FsRmdirNotFound", func(t *testing.T) {
    skipIfNotOk(t)
    if isBucketBasedButNotRoot(f) {
        t.Skip("Skipping test as non root bucket-based remote")
    }
    err := f.Rmdir(ctx, "")
    assert.Error(t, err, "Expecting error on Rmdir nonexistent")
})
```

**Key Points**:
- Test is called BEFORE any directory is created (line 529 shows `Mkdir` is called AFTER)
- Test calls `Rmdir("")` on root directory that doesn't exist
- Test expects ANY error (not specifically `fs.ErrorDirNotFound`)
- Test uses `assert.Error()` which just checks that `err != nil`

## Current Implementation Behavior

**Our Rmdir Implementation** (`raid3.go:1227-1278`):
1. Checks all backends available (health check)
2. Calls Rmdir on all 3 backends in parallel
3. Collects errors from all backends
4. **Returns `nil` if ANY backend returns `nil`** (line 1273-1274)
5. Returns `fs.ErrorDirNotFound` if ALL backends return `ErrorDirNotFound`
6. Otherwise returns first error

**Problem**: When a directory doesn't exist, if one backend returns `nil` (idempotent behavior), we return `nil` overall, but the test expects an error.

## Local Backend Behavior

**Local Backend Rmdir** (`backend/local/local.go:850-863`):
```go
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
    localPath := f.localPath(dir)
    if fi, err := os.Stat(localPath); err != nil {
        return err  // Returns os.ErrNotExist when directory doesn't exist
    } else if !fi.IsDir() {
        return fs.ErrorIsFile
    }
    err := os.Remove(localPath)
    // ...
    return err
}
```

**Behavior**:
- When directory doesn't exist: `os.Stat()` returns `os.ErrNotExist`
- Local backend returns `os.ErrNotExist` directly (not translated to `fs.ErrorDirNotFound`)
- This is an error, so should cause our implementation to return an error

## Union Backend Reference Implementation

**Union Backend Rmdir** (`backend/union/union.go:127-144`):
```go
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
    upstreams, err := f.action(ctx, dir)
    if err != nil {
        // ... handle error
        return err
    }
    errs := Errors(make([]error, len(upstreams)))
    multithread(len(upstreams), func(i int) {
        err := upstreams[i].Rmdir(ctx, dir)
        if err != nil {
            errs[i] = fmt.Errorf("%s: %w", upstreams[i].Name(), err)
        }
    })
    return errs.Err()  // Returns error if ANY backend returned error
}
```

**Union Backend Errors.Err()** (`backend/union/errors.go:38-44`):
```go
func (e Errors) Err() error {
    ne := e.FilterNil()  // Remove nil errors
    if len(ne) == 0 {
        return nil  // All nil = success
    }
    return ne  // Any error = return errors
}
```

**Key Difference**:
- Union backend: Returns error if **ANY** backend returns error
- Our implementation: Returns `nil` if **ANY** backend returns `nil`

## Root Cause Analysis

### Root Cause Identified: Logic Error in Error Handling

**The Problem**: Our current implementation has inverted logic compared to union backend:

**Our Current Logic** (line 1272-1274):
```go
// If at least one succeeded (returned nil), that's success
if evenErr == nil || oddErr == nil || parityErr == nil {
    return nil
}
```

**Union Backend Logic** (`backend/union/errors.go:38-44`):
```go
func (e Errors) Err() error {
    ne := e.FilterNil()  // Remove nil errors
    if len(ne) == 0 {
        return nil  // All nil = success
    }
    return ne  // Any error = return errors
}
```

**Key Difference**:
- **Union backend**: Returns error if **ANY** backend returns error (only returns `nil` if ALL are `nil`)
- **Our implementation**: Returns `nil` if **ANY** backend returns `nil` (even if others return errors)

**Why This Causes the Test Failure**:
1. Test calls `Rmdir("")` on non-existent root directory
2. All three backends should return errors (`os.ErrNotExist` or `fs.ErrorDirNotFound`)
3. But if ANY backend returns `nil` (idempotent behavior), we return `nil` overall
4. Test expects an error, but gets `nil` → test fails

**Evidence**:
- Union backend uses `Errors.Err()` which filters nil and returns error if any remain
- Our code returns `nil` if any backend returns `nil`, which is incorrect behavior
- The test expects an error when directory doesn't exist (standard rclone behavior)

## Investigation Steps

### Step 1: Check What Backends Actually Return
Create a test that calls `Rmdir("")` on each backend individually when root doesn't exist:

```go
// Test what each backend returns
evenBackend.Rmdir(ctx, "")  // What does this return?
oddBackend.Rmdir(ctx, "")   // What does this return?
parityBackend.Rmdir(ctx, "") // What does this return?
```

### Step 2: Check Root Directory State
Verify if root directories exist before the test:

```go
// Check if root exists
_, err := evenBackend.List(ctx, "")
// Does root exist or not?
```

### Step 3: Check Health Check Impact
Verify if `checkAllBackendsAvailable()` creates directories:

```go
// Does health check create root?
f.checkAllBackendsAvailable(ctx)
// Check if root exists after health check
```

### Step 4: Compare with Union Backend
Run the same test with union backend to see if it passes:

```go
// Does union backend pass this test?
// What does union backend return in this scenario?
```

## Expected Behavior

Based on rclone conventions and union backend behavior:

**When `Rmdir("")` is called on non-existent root**:
- If ALL backends return `ErrorDirNotFound`: Return `fs.ErrorDirNotFound`
- If ANY backend returns an error (including `os.ErrNotExist`): Return that error
- If ALL backends return `nil`: Return `nil` (success)

**Current Implementation Issue**:
- We return `nil` if ANY backend returns `nil`, even if others return errors
- This is incorrect - we should return an error if ANY backend returns an error

## Proposed Fix Strategy

### Recommended Approach: Match Union Backend Behavior

**Rationale**:
- Union backend is the reference implementation for multi-backend operations
- Proven pattern that works correctly
- Handles all error cases properly
- No performance overhead (no extra List() calls)

**Implementation**:

Change the error handling logic to:
1. Collect all errors from all backends
2. Filter out `nil` errors
3. If all errors are "not found" type (`fs.ErrorDirNotFound` or `os.ErrNotExist`), return `fs.ErrorDirNotFound`
4. If any non-"not found" errors exist, return the first error
5. Only return `nil` if ALL backends returned `nil` (all succeeded)

**Code Changes**:

```go
// After collecting evenErr, oddErr, parityErr from all backends:

// Collect non-nil errors
var allErrors []error
if evenErr != nil {
    allErrors = append(allErrors, evenErr)
}
if oddErr != nil {
    allErrors = append(allErrors, oddErr)
}
if parityErr != nil {
    allErrors = append(allErrors, parityErr)
}

// If all backends returned errors, check if they're all "not found"
if len(allErrors) == 3 {
    allNotFound := true
    for _, err := range allErrors {
        // Check for both fs.ErrorDirNotFound and os.ErrNotExist
        if !errors.Is(err, fs.ErrorDirNotFound) && !os.IsNotExist(err) {
            allNotFound = false
            break
        }
    }
    if allNotFound {
        return fs.ErrorDirNotFound
    }
}

// If any error exists, return first error (union backend pattern)
if len(allErrors) > 0 {
    return allErrors[0]
}

// All succeeded (all returned nil)
return nil
```

**Key Changes**:
- Remove the early return `if evenErr == nil || oddErr == nil || parityErr == nil { return nil }`
- Collect all errors first, then decide what to return
- Only return `nil` if ALL backends returned `nil`
- Handle both `fs.ErrorDirNotFound` and `os.ErrNotExist` as "not found" errors

## Next Steps

1. **Implement Fix**: Apply the recommended approach (match union backend behavior)
2. **Test**: Verify `TestStandard/FsRmdirNotFound` passes
3. **Verify**: Ensure other Rmdir tests still pass
4. **Document**: Update code comments to explain the error handling logic

## Summary

**Root Cause**: Logic error - we return `nil` if ANY backend returns `nil`, but should return error if ANY backend returns error.

**Fix**: Match union backend behavior - collect all errors, filter nil, return error if any remain, only return `nil` if all succeeded.

**Impact**: Low risk - only affects error handling logic, doesn't change core functionality.
