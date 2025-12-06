# Two-Phase Commit Explanation: Making Move Atomic

**Date**: December 4, 2025  
**Context**: Option 3 for fixing partial move problem in level3 backend

---

## 🎯 The Problem

**Current Move Operation**:
```
Time 1: Move even particle   → ✅ SUCCESS (completed)
Time 2: Move odd particle     → ✅ SUCCESS (completed)
Time 3: Move parity particle  → ❌ FAILED (backend unavailable)

Result: Partial move! Even and odd particles at new location,
        parity particle still at old location. INCONSISTENT STATE!
```

**Why this happens**:
- All three moves happen in parallel
- No coordination between backends
- Once a move completes, it can't be easily undone
- Error happens AFTER some moves already succeeded

---

## 💡 Solution: Two-Phase Commit

**Two-Phase Commit** is a distributed transaction protocol that ensures:
- **Atomicity**: Either ALL operations succeed, or NONE do
- **Consistency**: System is always in a valid state
- **Coordination**: All participants agree before committing

---

## 📋 How Two-Phase Commit Works

### Phase 1: Prepare (Lock/Verify)

**Goal**: Verify that ALL backends are ready to perform the move BEFORE doing anything destructive.

**What happens**:

1. **Lock Resources**:
   ```
   Even backend:   Lock source file, verify destination path available
   Odd backend:    Lock source file, verify destination path available
   Parity backend: Lock source file, verify destination path available
   ```

2. **Verify Conditions**:
   ```
   - Source file exists on backend? ✅
   - Destination path is available? ✅
   - Backend has space? ✅
   - Backend is writable? ✅
   - Network connection stable? ✅
   ```

3. **Reserve Resources** (if needed):
   ```
   - Create temporary "reservation" markers
   - Allocate space for destination
   - Pre-validate permissions
   ```

4. **Report Status**:
   ```
   Each backend responds:
   - ✅ "PREPARED" - Ready to commit
   - ❌ "ABORT" - Cannot proceed
   ```

**Key Point**: **NO ACTUAL MOVE HAPPENS YET!** We're just verifying readiness.

---

### Phase 2: Commit (Actual Move)

**Goal**: Perform the actual move operations only if ALL backends prepared successfully.

**What happens**:

1. **Check All Responses**:
   ```
   If ALL backends returned "PREPARED":
     → Proceed to Phase 2
   
   If ANY backend returned "ABORT":
     → Abort entire transaction
     → Release all locks
     → Return error
   ```

2. **Execute Moves** (if all prepared):
   ```
   Even backend:   Perform actual move ✅
   Odd backend:    Perform actual move ✅
   Parity backend: Perform actual move ✅
   ```

3. **Confirm Completion**:
   ```
   All backends confirm move completed
   Release locks
   Transaction complete ✅
   ```

---

## 🔍 Detailed Example: Lock/Verify Phase

### Current Problem

```go
// Current implementation (parallel moves)
g.Go(func() error {
    // Move even - happens immediately
    return f.even.Features().Move(ctx, obj, remote)
})

g.Go(func() error {
    // Move odd - happens immediately  
    return f.odd.Features().Move(ctx, obj, remote)
})

g.Go(func() error {
    // Move parity - happens immediately
    return f.parity.Features().Move(ctx, obj, remote)
})

// Problem: If parity fails, even and odd already moved!
```

### Two-Phase Commit Solution

```go
// PHASE 1: Prepare (lock/verify)
type PrepareResult struct {
    Backend string
    Ready   bool
    Error   error
}

prepareChan := make(chan PrepareResult, 3)

// Prepare even
g.Go(func() error {
    // LOCK: Verify source exists and is locked
    srcObj, err := f.even.NewObject(ctx, srcRemote)
    if err != nil {
        prepareChan <- PrepareResult{"even", false, err}
        return nil
    }
    
    // VERIFY: Check destination is available
    destExists, _ := f.even.NewObject(ctx, destRemote)
    if destExists != nil {
        prepareChan <- PrepareResult{"even", false, fs.ErrorFileExists}
        return nil
    }
    
    // VERIFY: Backend is writable (health check)
    if err := f.even.Mkdir(ctx, ".level3-prepare-check"); err != nil {
        prepareChan <- PrepareResult{"even", false, err}
        return nil
    }
    _ = f.even.Rmdir(ctx, ".level3-prepare-check")
    
    // All checks passed - ready to commit
    prepareChan <- PrepareResult{"even", true, nil}
    return nil
})

// Prepare odd and parity similarly...

// Wait for all prepare results
var allReady = true
for i := 0; i < 3; i++ {
    result := <-prepareChan
    if !result.Ready {
        allReady = false
        log.Printf("Backend %s not ready: %v", result.Backend, result.Error)
        break
    }
}

// PHASE 2: Commit (only if all prepared)
if !allReady {
    // Abort - release any locks, return error
    return fmt.Errorf("move aborted: not all backends ready")
}

// Now perform actual moves (all backends verified ready)
g.Go(func() error {
    return f.even.Features().Move(ctx, obj, remote)
})
// ... same for odd and parity
```

---

## 🔒 What "Lock/Verify" Means

### Lock

**Locking** prevents other operations from interfering:

1. **Source Lock**: 
   - Mark source file as "being moved"
   - Prevent other operations from modifying/deleting it
   - Ensures file won't disappear during move

2. **Destination Lock**:
   - Reserve destination path
   - Prevent other operations from creating file there
   - Ensures destination stays available

**Implementation Options**:
- **Explicit locks**: Backend provides lock mechanism (if available)
- **Atomic operations**: Use backend's atomic rename (most backends support this)
- **Temporary markers**: Create `.level3-moving-{uuid}` file as lock

### Verify

**Verification** checks all preconditions:

1. **Source Verification**:
   - ✅ Source file exists
   - ✅ Source file is readable
   - ✅ Source file not corrupted
   - ✅ Source file size matches expected

2. **Destination Verification**:
   - ✅ Destination path doesn't exist (or is empty)
   - ✅ Destination directory exists and is writable
   - ✅ Sufficient space available
   - ✅ Permissions allow move

3. **Backend Verification**:
   - ✅ Backend is accessible
   - ✅ Backend is writable
   - ✅ Network connection is stable
   - ✅ Backend not in error state

4. **Consistency Verification**:
   - ✅ All three backends report same source file exists
   - ✅ All three backends can access their particles
   - ✅ All three backends ready for move

---

## 🎯 Benefits of Two-Phase Commit

### 1. **Atomicity Guaranteed**
```
Either:
  - ALL moves succeed (all backends prepared + committed)
  - NO moves succeed (any backend failed prepare = abort)

No partial states possible!
```

### 2. **Fail Fast**
```
If backend unavailable:
  Phase 1 fails → Abort immediately
  No moves attempted
  No cleanup needed
  Clean error message
```

### 3. **Resource Safety**
```
Locks prevent:
  - Source file deleted during move
  - Destination created by another operation
  - Concurrent moves on same file
  - Race conditions
```

### 4. **Consistency**
```
All backends verified ready before ANY move happens
Either all succeed or all abort
System always in consistent state
```

---

## ⚠️ Challenges & Limitations

### Challenge 1: Backend Lock Support

**Problem**: Not all backends support locking
- S3/MinIO: No native locks
- Local filesystem: File locking available
- Cloud storage: Usually no locks

**Solution**: Use "optimistic locking" or temporary markers:
```go
// Create lock marker file
lockFile := ".level3-move-lock-" + uuid.New()
f.even.Put(ctx, bytes.NewReader([]byte{}), lockFile)

// After move, delete lock
f.even.Remove(ctx, lockFile)
```

### Challenge 2: Lock Timeout

**Problem**: What if prepare succeeds but commit fails?
- Locks held indefinitely?
- Resources reserved but never used?

**Solution**: Lock expiration/timeout:
```go
// Lock expires after 30 seconds
// If commit doesn't happen, locks auto-release
// Periodic cleanup removes stale locks
```

### Challenge 3: Performance Overhead

**Problem**: Two-phase commit adds latency:
- Phase 1: Prepare (100-200ms overhead)
- Phase 2: Commit (normal move time)

**Solution**: Acceptable trade-off for consistency:
- Better to be slow and correct than fast and broken
- Most operations already have network latency
- Overhead is one-time per operation

### Challenge 4: Complexity

**Problem**: Much more complex than current implementation

**Solution**: 
- Complexity is justified for data integrity
- Can be implemented incrementally
- Well-understood pattern (standard in distributed systems)

---

## 🔄 Alternative: Simplified Two-Phase Commit

For level3, we could use a **simplified version**:

### Phase 1: Pre-Move Validation (No Explicit Locks)

```go
// Just verify all backends are ready
func (f *Fs) prepareMove(ctx context.Context, src, dst string) error {
    // Check all backends can access source
    evenObj, errEven := f.even.NewObject(ctx, src)
    oddObj, errOdd := f.odd.NewObject(ctx, src)
    parityObj, errParity := f.parity.NewObject(ctx, src)
    
    if errEven != nil || errOdd != nil || errParity != nil {
        return fmt.Errorf("source not accessible on all backends")
    }
    
    // Verify destination available on all backends
    // (check doesn't exist or is empty)
    
    return nil // All ready
}
```

### Phase 2: Atomic Move (Use Backend's Atomic Operations)

```go
// Use backend's native atomic rename
// Most backends (S3, local) support atomic rename
// If rename succeeds, it's atomic (all-or-nothing)
```

**Trade-off**: Less protection (no explicit locks), but simpler implementation.

---

## 📊 Comparison with Current Approach

| Aspect | Current (Parallel Moves) | Two-Phase Commit |
|--------|-------------------------|------------------|
| **Speed** | ✅ Fast (all parallel) | ⚠️ Slower (+prepare phase) |
| **Atomicity** | ❌ No (partial moves possible) | ✅ Yes (all or nothing) |
| **Consistency** | ❌ Can create inconsistent state | ✅ Always consistent |
| **Complexity** | ✅ Simple | ⚠️ More complex |
| **Error Handling** | ❌ Partial failures messy | ✅ Clean abort/retry |
| **Data Safety** | ⚠️ Can leave orphaned files | ✅ Safe (no orphaned files) |

---

## ✅ Recommendation

For level3 backend, **simplified two-phase commit** is a good middle ground:

1. **Phase 1**: Enhanced pre-flight check (verify all backends ready)
2. **Phase 2**: Use backend's atomic rename operations

This provides:
- ✅ Better consistency than current approach
- ✅ Less complexity than full two-phase commit
- ✅ Reasonable performance overhead
- ✅ Works with existing backend capabilities

---

## 🔗 References

- **Two-Phase Commit Protocol**: Standard distributed transaction protocol
- **ACID Properties**: Atomicity, Consistency, Isolation, Durability
- **Distributed Systems**: Consensus protocols (Raft, Paxos)

---

## 📝 Summary

**"Prepare move on all backends (lock/verify)"** means:

1. **Before doing ANY actual move operations**:
   - Lock source files (prevent deletion/modification)
   - Verify destination paths are available
   - Check all backends are accessible and ready
   - Reserve resources if needed

2. **Only if ALL backends prepare successfully**:
   - Proceed to actual move operations
   - All moves happen (or none do)

3. **If ANY backend fails to prepare**:
   - Abort entire transaction
   - Release all locks
   - Return error
   - NO moves attempted

**Result**: Atomic move - either all particles move, or none do. No partial moves possible!


