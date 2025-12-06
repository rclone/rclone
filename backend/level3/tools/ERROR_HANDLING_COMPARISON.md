# Error Handling Comparison: Chunker, Union, and Level3

**Date**: December 4, 2025  
**Purpose**: Understand how other rclone backends handle partial failures

---

## 🎯 Overview

Comparing error handling strategies across:
- **Chunker backend**: Splits files into chunks
- **Union backend**: Mirrors data across multiple remotes
- **Level3 backend**: RAID 3 with even/odd/parity particles

---

## 📋 Chunker Backend: **Rollback Strategy**

### Error Scenario
**Problem**: Not all chunks can be uploaded (some chunks fail)

### How Chunker Handles It

**Strategy**: ✅ **Rollback - Clean up all uploaded chunks**

```go
// chunker.go:1144-1178
func (f *Fs) put(...) (obj fs.Object, err error) {
    var metaObject fs.Object
    defer func() {
        if err != nil {
            c.rollback(ctx, metaObject)  // ✅ Rollback on error!
        }
    }()
    
    // Upload chunks one by one
    for c.chunkNo = 0; !c.done; c.chunkNo++ {
        chunk, errChunk := basePut(ctx, wrapIn, info, options...)
        if errChunk != nil {
            return nil, errChunk  // Defer triggers rollback
        }
        c.chunks = append(c.chunks, chunk)
    }
    
    // ... finalize ...
}
```

**Rollback Implementation**:
```go
// chunker.go:1487-1497
func (c *chunkingReader) rollback(ctx context.Context, metaObject fs.Object) {
    if metaObject != nil {
        c.chunks = append(c.chunks, metaObject)
    }
    // Remove all uploaded chunks
    for _, chunk := range c.chunks {
        if err := chunk.Remove(ctx); err != nil {
            fs.Errorf(chunk, "Failed to remove temporary chunk: %v", err)
        }
    }
}
```

**Key Points**:
- ✅ **Defers rollback** - Automatically cleans up on any error
- ✅ **Removes all chunks** - Deletes all uploaded chunks
- ✅ **Best effort cleanup** - Logs errors but continues cleanup
- ✅ **No partial files** - Either all chunks uploaded, or none

### Move/Copy Operations

```go
// chunker.go:1701-1727
// Copy/move chunks sequentially
for _, chunk := range o.chunks {
    chunkResult, err := do(ctx, chunk, remote+chunkSuffix)
    if err != nil {
        break  // Stop on first error
    }
    newChunks = append(newChunks, chunkResult)
}

// If error, rollback successful moves
if err != nil {
    for _, chunk := range newChunks {
        silentlyRemove(ctx, chunk)  // ✅ Rollback!
    }
    return nil, err
}
```

**Behavior**: ✅ **Rollback on failure** - Removes all successfully moved chunks

---

## 📋 Union Backend: **Aggregate Errors (No Rollback)**

### Error Scenario
**Problem**: Not all remotes receive data properly (some remotes fail)

### How Union Handles It

**Strategy**: ⚠️ **Allow Partial Success - Aggregate Errors**

```go
// union.go:533-586
func (f *Fs) put(...) (fs.Object, error) {
    // Upload to multiple upstreams in parallel
    errs := Errors(make([]error, len(upstreams)+1))
    objs := make([]upstream.Entry, len(upstreams))
    
    multithread(len(upstreams), func(i int) {
        u := upstreams[i]
        o, err := u.Put(ctx, readers[i], src, options...)
        if err != nil {
            errs[i] = fmt.Errorf("%s: %w", u.Name(), err)
            // Drain buffer to allow other uploads to continue
            _, _ = io.Copy(io.Discard, readers[i])
            return  // ⚠️ Continue with other upstreams
        }
        objs[i] = u.WrapObject(o)  // Success - keep it
    })
    
    errs[len(upstreams)] = <-errChan
    err = errs.Err()  // Aggregate all errors
    
    if err != nil {
        return nil, err  // ⚠️ BUT: Successful uploads remain!
    }
    
    return f.wrapEntries(objs...), nil
}
```

**Key Points**:
- ⚠️ **No rollback** - Successful uploads remain even if others fail
- ⚠️ **Partial success allowed** - Some remotes may have file, others don't
- ✅ **Error aggregation** - Returns combined error message
- ✅ **Continues on error** - Other uploads continue even if one fails

**Error Aggregation**:
```go
// union/errors.go:36-43
func (e Errors) Err() error {
    ne := e.FilterNil()  // Remove nil errors
    if len(ne) == 0 {
        return nil
    }
    return ne  // Return combined error
}
```

### Update Operations

```go
// union/entry.go:89-106
multithread(len(entries), func(i int) {
    err := o.Update(ctx, readers[i], src, options...)
    if err != nil {
        errs[i] = fmt.Errorf("%s: %w", o.UpstreamFs().Name(), err)
        // Drain buffer - allow other updates to continue
        _, _ = io.Copy(io.Discard, readers[i])
    }
})

errs[len(entries)] = <-errChan
return errs.Err()  // ⚠️ No rollback - partial updates remain
```

**Behavior**: ⚠️ **No rollback** - Successful updates remain, failed ones return error

---

## 📊 Comparison Table

| Aspect | Chunker | Union | Level3 (Current) | Level3 (With Rollback) |
|--------|---------|-------|------------------|------------------------|
| **Strategy** | ✅ Rollback | ⚠️ Aggregate errors | ❌ No rollback | ✅ Rollback (proposed) |
| **Partial Success** | ❌ Not allowed | ✅ Allowed | ❌ Should not allow | ❌ Not allowed |
| **Cleanup** | ✅ Removes all chunks | ❌ No cleanup | ❌ No cleanup | ✅ Rollback successful |
| **Error Handling** | Clean error | Aggregated errors | First error | Clean error |
| **Data Consistency** | ✅ Consistent | ⚠️ Can be inconsistent | ❌ Inconsistent (partial moves) | ✅ Consistent |
| **Complexity** | Medium | Low | Low | Medium |

---

## 🔍 Detailed Comparison

### 1. **Put/Create Operations**

#### Chunker: Rollback ✅
```
Upload chunk 1: ✅ Success
Upload chunk 2: ✅ Success
Upload chunk 3: ❌ Failed

Result: 
  - All chunks removed (rollback)
  - Clean error returned
  - No partial file
```

#### Union: Partial Success ⚠️
```
Upload to remote1: ✅ Success
Upload to remote2: ❌ Failed
Upload to remote3: ✅ Success

Result:
  - Remote1 has file ✅
  - Remote2 failed ❌
  - Remote3 has file ✅
  - Error returned (but file exists on some remotes)
```

#### Level3 (Current): Partial Success ❌
```
Upload even:   ✅ Success
Upload odd:    ❌ Failed
Upload parity: ✅ Success

Result:
  - Partial file created (even + parity)
  - Error returned
  - File in degraded state
```

#### Level3 (With Rollback): Clean Failure ✅
```
Upload even:   ✅ Success
Upload odd:    ❌ Failed
Upload parity: ✅ Success

Result:
  - Even and parity rolled back
  - Clean error returned
  - No partial file
```

---

### 2. **Move Operations**

#### Chunker: Rollback ✅
```go
// chunker.go:1701-1727
for _, chunk := range o.chunks {
    chunkResult, err := do(ctx, chunk, remote+chunkSuffix)
    if err != nil {
        break
    }
    newChunks = append(newChunks, chunkResult)
}

if err != nil {
    // ✅ Rollback: Remove successfully moved chunks
    for _, chunk := range newChunks {
        silentlyRemove(ctx, chunk)
    }
    return nil, err
}
```

#### Union: No Move Support
- Union doesn't implement Move operations
- Uses copy + delete pattern (not atomic)

#### Level3 (Current): No Rollback ❌
```go
// Moves happen in parallel
// If one fails, others remain moved
// Result: Partial move
```

#### Level3 (With Rollback): Rollback ✅
```go
// Track successful moves
// On error, move particles back
// Result: All-or-nothing
```

---

## 🤔 Why Different Strategies?

### Chunker: Why Rollback?

**Reason**: Chunks are interdependent
- File is incomplete without all chunks
- Partial file is useless
- Metadata requires all chunks
- **Must be all-or-nothing**

**Philosophy**: "If we can't upload everything, upload nothing"

### Union: Why No Rollback?

**Reason**: Remotes are independent
- Each remote has complete file
- One remote failing doesn't affect others
- User might want file on available remotes
- **Partial success is acceptable**

**Philosophy**: "Put file wherever possible"

### Level3: Why Should Rollback?

**Reason**: RAID 3 requires all particles
- File needs all 3 particles (or reconstruction)
- Partial file = degraded state
- Hardware RAID 3 blocks writes in degraded mode
- **Must be all-or-nothing**

**Philosophy**: "Strict write policy - all or nothing"

---

## 💡 Key Insights for Level3

### 1. **Chunker's Approach is Closest**

**Similarity**:
- ✅ Both split data across multiple backends
- ✅ Both require all parts for complete file
- ✅ Both use rollback on failure
- ✅ Both aim for all-or-nothing

**Key Pattern**: **Defer-based rollback**
```go
defer func() {
    if err != nil {
        rollback()
    }
}()
```

### 2. **Union's Approach is Different**

**Difference**:
- ⚠️ Union allows partial success (by design)
- ⚠️ Level3 should NOT allow partial success (RAID 3 strict policy)
- ✅ Union is about redundancy (multiple copies)
- ✅ Level3 is about striping (one logical file)

### 3. **Level3 Should Follow Chunker's Pattern**

**Recommended**:
- ✅ Use rollback like chunker
- ✅ Clean up all particles on failure
- ✅ All-or-nothing guarantee
- ✅ Defer-based cleanup

---

## 📝 Implementation Recommendations

### For Level3 Move Operation

**Follow Chunker's Pattern**:

```go
func (f *Fs) Move(...) (fs.Object, error) {
    var successMoves []moveState
    
    // Attempt moves
    // Track successful moves
    
    if err != nil {
        // Rollback successful moves (like chunker)
        for _, move := range successMoves {
            // Move back or delete
        }
        return nil, err
    }
    
    return newObj, nil
}
```

**Key Elements from Chunker**:
1. ✅ Track what succeeded
2. ✅ Rollback on error
3. ✅ Best-effort cleanup
4. ✅ Return clean error

---

## ✅ Conclusion

**Chunker**: ✅ Uses rollback - perfect example for level3  
**Union**: ⚠️ Allows partial success - NOT suitable for level3  
**Level3**: Should implement rollback like chunker

**Recommendation**: Implement rollback for Move operations, following chunker's pattern of tracking success and cleaning up on failure.

---

## 🔗 Related Files

- `backend/chunker/chunker.go` - Lines 1487-1497 (rollback), 1723-1727 (move rollback)
- `backend/union/union.go` - Lines 533-586 (put), 581-583 (error handling)
- `backend/level3/level3.go` - Lines 2122-2212 (current Move implementation)


