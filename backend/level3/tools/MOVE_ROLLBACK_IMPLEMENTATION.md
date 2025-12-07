# Move Rollback Implementation Guide

**Date**: December 4, 2025  
**Goal**: Implement rollback for Move operations to prevent partial moves

---

## 🔍 Problem Analysis

### What the Test Detects

The test `compare_level3_with_single_errors.sh test move-fail-even` correctly detects that **partial moves occur** when a backend becomes unavailable during a move operation.

#### Test Results

```
✓ Move command returns non-zero exit code (move failed)
✗ New file exists at destination (partial move occurred)
  - Odd particle moved to new location
  - Parity particle moved to new location
  - Even particle failed (backend unavailable)
```

### Expected Behavior

According to RAID 3 strict write policy (documented in `ERROR_HANDLING_POLICY.md`):

1. **Pre-flight Check**: Move should fail at `checkAllBackendsAvailable()` if any backend is unavailable
2. **No Partial Moves**: If move starts, all particles must move atomically
3. **Rollback**: If move fails partway, completed moves should be rolled back

### Actual Behavior

The test reveals:

1. **Move command fails** (non-zero exit) ✓
2. **But partial move occurs** (some particles moved) ✗
3. **No rollback** (completed moves not undone) ✗

#### Error Sequence

```
1. Backend stopped (even backend unavailable)
2. Move command attempted
3. Error: HEAD operation fails (checking destination)
   "dial tcp 127.0.0.1:9001: connect: connection refused"
4. Move command returns error (exit code non-zero)
5. BUT: Odd and parity particles already at new location!
```

### Root Cause Analysis

#### Why Does Partial Move Occur?

1. **Pre-flight check timing**: The `checkAllBackendsAvailable()` check happens, but:
   - Backend might be reachable during check
   - Backend fails between check and move
   - Race condition between check and move operations

2. **Rclone operations layer**: 
   - Does HEAD check before calling backend Move()
   - HEAD check fails (backend unavailable)
   - But Move() might have already been attempted
   - Or particles moved before HEAD check completed

3. **No rollback mechanism**:
   - Move operations don't track which particles moved successfully
   - No mechanism to undo completed moves
   - Error is returned, but state is inconsistent

**Current Implementation** (no rollback):
```go
// Move all three particles in parallel
g.Go(func() error { /* move even */ })
g.Go(func() error { /* move odd */ })
g.Go(func() error { /* move parity */ })

err := g.Wait()
if err != nil {
    return nil, err  // ❌ Problem: Already-completed moves not undone!
}
```

**What happens**:
```
Move even:   ✅ Completed before error
Move odd:    ❌ Failed (backend unavailable)
Move parity: 🔄 Cancelled by context

Result: Even particle at new location, odd/parity at old location
        → INCONSISTENT STATE!
```

### Known Limitations

From `ERROR_HANDLING_POLICY.md` (lines 186-219):

#### Problem: No Rollback for Move

```
Move even:   ✅ Completed before error
Move odd:    ❌ Failed
Move parity: 🔄 Cancelled by context
```

**Result**: Even particle at new location, odd/parity at old location!

#### Current Implementation

- Uses `errgroup` for parallel moves
- Context cancellation stops pending moves
- **BUT**: Already-completed moves aren't undone

---

## 🎯 Strategy: "If Any Error, Just Rollback"

**Simple Rule**: 
- Track which backend moves succeeded
- If ANY backend move fails, rollback all successful moves
- Return error to user

**Result**: All-or-nothing guarantee - either all particles move, or none do.

---

## ✅ Solution: Track & Rollback

### Implementation Approach

1. **Track successful moves** - Record which backends moved successfully
2. **On failure** - Rollback all successful moves
3. **Return error** - User sees clean error, no partial state

---

## 🔧 Implementation Details

### Step 1: Track Success State

Use channels or atomic flags to track which moves succeeded:

```go
type moveResult struct {
    backend string
    success bool
    err     error
}

results := make(chan moveResult, 3)
```

### Step 2: Move with Tracking

```go
// Move even
g.Go(func() error {
    obj, err := f.even.NewObject(ctx, srcObj.remote)
    if err != nil {
        results <- moveResult{"even", false, nil} // Not found = skip
        return nil
    }
    if do := f.even.Features().Move; do != nil {
        _, err = do(ctx, obj, remote)
        if err == nil {
            results <- moveResult{"even", true, nil}
        } else {
            results <- moveResult{"even", false, err}
        }
        return err
    }
    results <- moveResult{"even", false, fs.ErrorCantMove}
    return fs.ErrorCantMove
})

// Similar for odd and parity...
```

### Step 3: Check Results & Rollback

```go
// Wait for all moves
err := g.Wait()

// Collect results
successMap := make(map[string]bool)
var firstError error
for i := 0; i < 3; i++ {
    result := <-results
    if result.success {
        successMap[result.backend] = true
    } else if result.err != nil && firstError == nil {
        firstError = result.err
    }
}

// If any failed, rollback successful moves
if firstError != nil {
    rollbackErr := f.rollbackMoves(ctx, srcObj.remote, remote, successMap)
    if rollbackErr != nil {
        // Log rollback error but return original error
        fs.Errorf(f, "Rollback failed: %v", rollbackErr)
    }
    return nil, firstError
}

// All succeeded
return &Object{fs: f, remote: remote}, nil
```

### Step 4: Rollback Function

```go
func (f *Fs) rollbackMoves(ctx context.Context, srcRemote, dstRemote string, 
    successMap map[string]bool) error {
    
    // Rollback in parallel (best effort - don't fail if rollback fails)
    g, _ := errgroup.WithContext(ctx)
    
    if successMap["even"] {
        g.Go(func() error {
            // Move even back from destination to source
            dstObj, err := f.even.NewObject(ctx, dstRemote)
            if err != nil {
                return nil // Already rolled back or doesn't exist
            }
            if do := f.even.Features().Move; do != nil {
                _, err := do(ctx, dstObj, srcRemote)
                // Don't return error - best effort rollback
                if err != nil {
                    fs.Errorf(f, "Rollback even failed: %v", err)
                }
            }
            return nil
        })
    }
    
    if successMap["odd"] {
        g.Go(func() error {
            // Move odd back...
            return nil
        })
    }
    
    if successMap["parity"] {
        g.Go(func() error {
            // Move parity back...
            return nil
        })
    }
    
    _ = g.Wait() // Don't return error - best effort
    
    return nil
}
```

---

## 🔄 Rollback Strategy Options

### Option A: Move Back (Preferred)

**Approach**: Move particles back from destination to source

```go
// Move back: dstRemote → srcRemote
dstObj, err := f.even.NewObject(ctx, dstRemote)
if err == nil {
    f.even.Features().Move(ctx, dstObj, srcRemote)
}
```

**Pros**:
- ✅ Restores original state completely
- ✅ Preserves file data
- ✅ Clean rollback

**Cons**:
- ⚠️ Requires source location still valid
- ⚠️ Two move operations (forward + back)

### Option B: Delete from Destination

**Approach**: Delete particles from new location

```go
// Delete from destination
dstObj, err := f.even.NewObject(ctx, dstRemote)
if err == nil {
    f.even.Remove(ctx, dstObj)
}
```

**Pros**:
- ✅ Simple - just delete
- ✅ Fast

**Cons**:
- ❌ File disappears (original at source, deleted at destination)
- ❌ Not a true rollback (file might be partially moved)

### Option C: Hybrid (Recommended)

**Strategy**: Try move back first, fallback to delete if source invalid

```go
func (f *Fs) rollbackMove(ctx context.Context, backend fs.Fs, 
    srcRemote, dstRemote string) error {
    
    dstObj, err := backend.NewObject(ctx, dstRemote)
    if err != nil {
        return nil // Already rolled back or doesn't exist
    }
    
    // Try to move back
    if do := backend.Features().Move; do != nil {
        _, err := do(ctx, dstObj, srcRemote)
        if err == nil {
            return nil // Successfully moved back
        }
        // Move back failed - source might not be valid
        fs.Debugf(f, "Move back failed, deleting from destination: %v", err)
    }
    
    // Fallback: Delete from destination
    return backend.Remove(ctx, dstObj)
}
```

---

## 📊 Complete Implementation Example

Here's a complete refactored Move function with rollback:

```go
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
    // Pre-flight check: Enforce strict RAID 3 write policy
    if err := f.checkAllBackendsAvailable(ctx); err != nil {
        return nil, fmt.Errorf("move blocked in degraded mode (RAID 3 policy): %w", err)
    }

    ctx = f.disableRetriesForWrites(ctx)

    srcObj, ok := src.(*Object)
    if !ok {
        return nil, fs.ErrorCantMove
    }

    // Determine source parity name...
    var srcParityName string
    parityOddSrc := GetParityFilename(srcObj.remote, true)
    parityEvenSrc := GetParityFilename(srcObj.remote, false)
    // ... (existing logic)

    // Determine destination parity name
    _, isParity, isOddLength := StripParitySuffix(srcParityName)
    if !isParity {
        isOddLength = false
    }
    dstParityName := GetParityFilename(remote, isOddLength)

    // Track successful moves
    type moveState struct {
        backend string
        srcName string
        dstName string
        success bool
        err     error
    }
    
    results := make(chan moveState, 3)
    g, gCtx := errgroup.WithContext(ctx)

    // Move even
    g.Go(func() error {
        obj, err := f.even.NewObject(gCtx, srcObj.remote)
        if err != nil {
            results <- moveState{"even", srcObj.remote, remote, false, nil}
            return nil // Not found = skip
        }
        if do := f.even.Features().Move; do != nil {
            _, err = do(gCtx, obj, remote)
            results <- moveState{"even", srcObj.remote, remote, err == nil, err}
            return err
        }
        results <- moveState{"even", srcObj.remote, remote, false, fs.ErrorCantMove}
        return fs.ErrorCantMove
    })

    // Move odd
    g.Go(func() error {
        obj, err := f.odd.NewObject(gCtx, srcObj.remote)
        if err != nil {
            results <- moveState{"odd", srcObj.remote, remote, false, nil}
            return nil
        }
        if do := f.odd.Features().Move; do != nil {
            _, err = do(gCtx, obj, remote)
            results <- moveState{"odd", srcObj.remote, remote, err == nil, err}
            return err
        }
        results <- moveState{"odd", srcObj.remote, remote, false, fs.ErrorCantMove}
        return fs.ErrorCantMove
    })

    // Move parity
    g.Go(func() error {
        obj, err := f.parity.NewObject(gCtx, srcParityName)
        if err != nil {
            results <- moveState{"parity", srcParityName, dstParityName, false, nil}
            return nil
        }
        if do := f.parity.Features().Move; do != nil {
            _, err = do(gCtx, obj, dstParityName)
            results <- moveState{"parity", srcParityName, dstParityName, err == nil, err}
            return err
        }
        results <- moveState{"parity", srcParityName, dstParityName, false, fs.ErrorCantMove}
        return fs.ErrorCantMove
    })

    // Wait for all moves
    moveErr := g.Wait()

    // Collect results
    var successMoves []moveState
    var firstError error
    
    for i := 0; i < 3; i++ {
        state := <-results
        if state.success {
            successMoves = append(successMoves, state)
        } else if state.err != nil && firstError == nil {
            firstError = state.err
        }
    }

    // If any failed, rollback successful moves
    if firstError != nil || moveErr != nil {
        if len(successMoves) > 0 {
            rollbackErr := f.rollbackMoves(ctx, successMoves)
            if rollbackErr != nil {
                fs.Errorf(f, "Rollback failed (some particles may be at new location): %v", rollbackErr)
            }
        }
        // Return the original error
        if firstError != nil {
            return nil, firstError
        }
        return nil, moveErr
    }

    // All succeeded
    return &Object{
        fs:     f,
        remote: remote,
    }, nil
}

// Rollback successful moves
func (f *Fs) rollbackMoves(ctx context.Context, moves []moveState) error {
    g, _ := errgroup.WithContext(ctx)
    
    for _, move := range moves {
        move := move // Capture for goroutine
        g.Go(func() error {
            var backend fs.Fs
            switch move.backend {
            case "even":
                backend = f.even
            case "odd":
                backend = f.odd
            case "parity":
                backend = f.parity
            default:
                return nil
            }
            
            // Try to move back
            dstObj, err := backend.NewObject(ctx, move.dstName)
            if err != nil {
                return nil // Already rolled back or doesn't exist
            }
            
            if do := backend.Features().Move; do != nil {
                _, err := do(ctx, dstObj, move.srcName)
                if err == nil {
                    return nil // Successfully moved back
                }
                // Move back failed - try to delete from destination
                fs.Debugf(f, "Rollback move back failed for %s, deleting: %v", move.backend, err)
            }
            
            // Fallback: Delete from destination
            if err := backend.Remove(ctx, dstObj); err != nil {
                fs.Errorf(f, "Rollback delete failed for %s: %v", move.backend, err)
            }
            return nil
        })
    }
    
    _ = g.Wait() // Best effort - don't return errors
    return nil
}
```

---

## ⚠️ Edge Cases & Considerations

### 1. **Rollback Failures**

**Problem**: What if rollback itself fails?

**Solution**: Best-effort rollback
- Log errors but don't fail the operation
- User already sees original error
- Some particles might remain at destination (documented limitation)

### 2. **Source Already Deleted**

**Problem**: If source was deleted during move, can't move back

**Solution**: Fallback to delete from destination
- Try move back first
- If fails (source invalid), delete from destination
- At least clean up destination

### 3. **Concurrent Operations**

**Problem**: Another operation might modify files during rollback

**Solution**: 
- Pre-flight check should prevent this (backend unavailable)
- Rollback is best-effort
- Log warnings if rollback conflicts occur

### 4. **Performance Impact**

**Concern**: Rollback adds latency on failures

**Reality**: 
- Only happens on failures (uncommon)
- Better than leaving inconsistent state
- Acceptable trade-off for data consistency

---

## ✅ Benefits

1. **All-or-Nothing Guarantee**
   - Either all particles move, or none do
   - No partial moves possible

2. **Simple Logic**
   - Track success → Rollback on failure
   - Easier than two-phase commit

3. **Works with Existing Backends**
   - Uses standard Move/Remove operations
   - No backend changes needed

4. **Fails Cleanly**
   - User sees clear error
   - System state is consistent
   - No orphaned particles

---

## 📝 Testing

After implementing rollback, the test should:

1. ✅ Detect when moves fail
2. ✅ Verify no particles at destination (all rolled back)
3. ✅ Verify all particles still at source
4. ✅ Verify error is returned

The existing test `compare_level3_with_single_errors.sh test move-fail-even` should pass once rollback is implemented!

---

## 🎯 Next Steps

1. Implement rollback in `Move()` function
2. Add `rollbackMoves()` helper function
3. Test with existing error scenarios
4. Verify test passes: `compare_level3_with_single_errors.sh`
5. Document rollback behavior in README

---

## 🔗 Related Documentation

- `backend/level3/docs/ERROR_HANDLING_POLICY.md` - Official error handling policy
- `backend/level3/docs/STRICT_WRITE_FIX.md` - Strict write policy implementation
- `backend/level3/level3.go` - Move implementation (lines 2122-2212)
- `backend/level3/tools/compare_level3_with_single_errors.sh` - Test suite for move-fail scenarios

**Note**: Problem analysis is now included at the beginning of this document (see "Problem Analysis" section above).


