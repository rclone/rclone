# Self-Healing Research: Automatic Particle Reconstruction During Reads

**Date**: November 1, 2025  
**Feature**: Automatic reconstruction and upload of missing particles during degraded reads

---

## 🎯 Proposed Feature

When downloading an object in degraded mode (one particle missing):
1. **Download** and **reconstruct** the original object from two available particles
2. **Calculate** the missing third particle during reconstruction
3. **Upload** the missing particle in the background
4. Return the reconstructed data to the user

This would provide **automatic self-healing** similar to commercial RAID systems.

---

## 📋 Research Questions

### 1. Can `Object.Open()` perform writes to the backend?
### 2. Can we spawn background goroutines that continue after `Open()` returns?
### 3. What are the architectural constraints?
### 4. Are there existing patterns in rclone for this?

---

## 🔍 Research Findings

### Finding 1: `Object.Open()` Interface Constraints

**Interface Definition** (`fs/types.go:89-90`):
```go
// Open opens the file for read.  Call Close() on the returned io.ReadCloser
Open(ctx context.Context, options ...OpenOption) (io.ReadCloser, error)
```

**Key Observations**:
- ✅ `Open()` is **not restricted** to read-only operations
- ✅ The method can perform **any side effects** before returning the `ReadCloser`
- ✅ No architectural prohibition against writes during `Open()`
- ⚠️ The method signature expects **synchronous** return of a `ReadCloser`

**Verdict**: **Technically allowed**, but must return quickly.

---

### Finding 2: Background Operations in Rclone

#### Pattern 1: Cache Backend - Background Uploads

**File**: `backend/cache/handle.go:562-630`

The cache backend implements **background upload workers**:

```go
func (b *backgroundWriter) run() {
    for {
        // ... wait for pending uploads ...
        
        remote := b.fs.cleanRootFromPath(absPath)
        b.notify(remote, BackgroundUploadStarted, nil)
        fs.Infof(remote, "background upload: started upload")
        
        err = operations.MoveFile(context.TODO(), b.fs.UnWrap(), 
                                  b.fs.tempFs, remote, remote)
        
        if err != nil {
            b.notify(remote, BackgroundUploadError, err)
            // ... handle error ...
        }
        
        fs.Infof(remote, "background upload: uploaded entry")
        b.notify(remote, BackgroundUploadCompleted, nil)
    }
}
```

**Key Insights**:
- ✅ Uses a **persistent background goroutine** (started at Fs initialization)
- ✅ Uses a **queue** to track pending uploads
- ✅ Uploads happen **asynchronously** after reads complete
- ✅ Has **error handling** and **retry logic**
- ✅ Uses **notifications** to track upload state

**Lifecycle**: Background worker runs for the **entire lifetime** of the Fs.

---

#### Pattern 2: VFS Cache - Writeback Queue

**File**: `vfs/vfscache/writeback/writeback.go:1-65`

The VFS cache implements a **writeback queue**:

```go
type WriteBack struct {
    id      Handle
    ctx     context.Context
    mu      sync.Mutex
    items   writeBackItems            // priority queue
    lookup  map[Handle]*writeBackItem
    opt     *vfscommon.Options
    timer   *time.Timer
    expiry  time.Time
    uploads int
}
```

**Key Insights**:
- ✅ Uses a **priority queue** for pending writes
- ✅ Supports **delayed writeback** (configurable delay)
- ✅ Tracks **upload state** per item
- ✅ Handles **context cancellation** for cleanup
- ✅ Supports **multiple concurrent uploads**

**Lifecycle**: Created with the VFS cache, runs until context cancelled.

---

#### Pattern 3: Goroutines in `Close()`

**File**: `backend/ftp/ftp.go:1262-1300`

The FTP backend spawns a goroutine in `Close()`:

```go
func (f *ftpReadCloser) Close() error {
    var err error
    errchan := make(chan error, 1)
    
    go func() {
        errchan <- f.rc.Close()
    }()
    
    // Wait for Close for up to 60 seconds
    closeTimeout := f.f.opt.CloseTimeout
    timer := time.NewTimer(time.Duration(closeTimeout))
    
    select {
    case err = <-errchan:
        timer.Stop()
    case <-timer.C:
        fs.Errorf(f.f, "Timeout when waiting for connection Close")
        return nil
    }
    
    return err
}
```

**Key Insights**:
- ✅ Goroutines can be spawned in `Close()` for cleanup
- ✅ Uses **channels** for synchronization
- ✅ Has **timeout protection**
- ⚠️ Still **blocks** until completion or timeout

**Limitation**: Not truly asynchronous - waits for completion.

---

#### Pattern 4: Custom ReadCloser with State

**File**: `backend/mega/mega.go:1030-1110`

The MEGA backend returns a custom `ReadCloser` that manages state:

```go
type openObject struct {
    ctx    context.Context
    mu     sync.Mutex
    o      *Object
    d      *mega.Download
    id     int
    skip   int64
    chunk  []byte
    closed bool
}

func (oo *openObject) Read(p []byte) (n int, err error) {
    // ... downloads chunks on demand ...
}

func (oo *openObject) Close() (err error) {
    oo.mu.Lock()
    defer oo.mu.Unlock()
    if oo.closed {
        return nil
    }
    err = oo.d.Finish()  // Cleanup operation
    oo.closed = true
    return nil
}
```

**Key Insights**:
- ✅ Custom `ReadCloser` can hold **state** and **context**
- ✅ Can perform **operations in Close()**
- ✅ Thread-safe with **mutexes**
- ⚠️ Operations in `Close()` are **synchronous**

---

### Finding 3: Architectural Constraints

#### Constraint 1: Context Lifetime
- The `ctx` passed to `Open()` is **request-scoped**
- It may be **cancelled** when the read operation completes
- Background operations need a **separate context** (e.g., `context.Background()` or `context.TODO()`)

#### Constraint 2: Fs Lifetime
- Background goroutines must not outlive the `Fs` instance
- Need a mechanism to **stop background workers** when `Fs` is destroyed
- Cache backend uses a **context** passed to `NewFs` for this

#### Constraint 3: Error Handling
- Background uploads can **fail**
- User won't see the error (read already succeeded)
- Need **logging** and potentially **retry logic**

#### Constraint 4: Concurrency
- Multiple reads can happen **simultaneously**
- Need to avoid **duplicate uploads** of the same particle
- Need **synchronization** (e.g., map of in-progress uploads)

---

## 🏗️ Proposed Architecture

### Option 1: Background Worker (Recommended)

**Similar to cache backend's background upload worker.**

#### Components:

1. **Upload Queue**:
   ```go
   type uploadJob struct {
       remote       string
       particleType string  // "even", "odd", or "parity"
       data         []byte
       timestamp    time.Time
   }
   
   type uploadQueue struct {
       mu      sync.Mutex
       pending map[string]*uploadJob  // key: remote+particleType
       jobs    chan *uploadJob
   }
   ```

2. **Background Worker** (started in `NewFs`):
   ```go
   func (f *Fs) backgroundUploader(ctx context.Context) {
       for {
           select {
           case job := <-f.uploadQueue.jobs:
               f.uploadParticle(ctx, job)
           case <-ctx.Done():
               return
           }
       }
   }
   ```

3. **Integration in `Object.Open()`**:
   ```go
   func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
       // ... existing reconstruction logic ...
       
       if reconstructed {
           // Queue missing particle for upload
           o.fs.queueParticleUpload(o.remote, missingParticleType, missingData)
           fs.Infof(o, "Queued %s particle for self-healing upload", missingParticleType)
       }
       
       // Return reconstructed data immediately
       return io.NopCloser(bytes.NewReader(merged)), nil
   }
   ```

**Advantages**:
- ✅ **Non-blocking**: Read returns immediately
- ✅ **Robust**: Survives errors, has retry logic
- ✅ **Efficient**: Batches uploads, avoids duplicates
- ✅ **Clean lifecycle**: Tied to Fs context

**Disadvantages**:
- ⚠️ More complex implementation
- ⚠️ Requires queue management
- ⚠️ Upload happens **after** read completes (not during)

---

### Option 2: Goroutine in `Close()`

**Spawn a goroutine when the ReadCloser is closed.**

#### Implementation:

```go
type selfHealingReadCloser struct {
    io.ReadCloser
    fs           *Fs
    remote       string
    missingType  string
    missingData  []byte
    uploadOnce   sync.Once
}

func (rc *selfHealingReadCloser) Close() error {
    err := rc.ReadCloser.Close()
    
    // Spawn background upload (fire-and-forget)
    rc.uploadOnce.Do(func() {
        go func() {
            ctx := context.Background()
            uploadErr := rc.fs.uploadParticle(ctx, rc.remote, rc.missingType, rc.missingData)
            if uploadErr != nil {
                fs.Errorf(rc.fs, "Self-healing upload failed for %s (%s): %v", 
                         rc.remote, rc.missingType, uploadErr)
            } else {
                fs.Infof(rc.fs, "Self-healing upload completed for %s (%s)", 
                        rc.remote, rc.missingType)
            }
        }()
    })
    
    return err
}
```

**Advantages**:
- ✅ **Simple**: No queue management
- ✅ **Immediate**: Upload starts when read completes
- ✅ **Self-contained**: No global state

**Disadvantages**:
- ⚠️ **Fire-and-forget**: No retry logic
- ⚠️ **No deduplication**: Multiple reads = multiple uploads
- ⚠️ **Goroutine leak risk**: If many reads fail
- ⚠️ **No lifecycle management**: Goroutines may outlive Fs

---

### Option 3: Synchronous Upload in `Open()`

**Upload the missing particle before returning.**

#### Implementation:

```go
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
    // ... existing reconstruction logic ...
    
    if reconstructed {
        fs.Infof(o, "Uploading missing %s particle for self-healing", missingParticleType)
        
        err := o.fs.uploadParticle(ctx, o.remote, missingParticleType, missingData)
        if err != nil {
            fs.Errorf(o, "Self-healing upload failed: %v (continuing with read)", err)
            // Continue anyway - read still works
        } else {
            fs.Infof(o, "Self-healing upload completed")
        }
    }
    
    return io.NopCloser(bytes.NewReader(merged)), nil
}
```

**Advantages**:
- ✅ **Simplest**: No goroutines, no queues
- ✅ **Immediate**: Particle restored before read completes
- ✅ **Guaranteed**: Upload attempted every time

**Disadvantages**:
- ❌ **Blocking**: Slows down read operations significantly
- ❌ **Poor UX**: User waits for upload during read
- ❌ **Timeout risk**: May hit context timeout
- ❌ **Not scalable**: Multiple concurrent reads = multiple concurrent uploads

**Verdict**: **Not recommended** - defeats the purpose of fast degraded reads.

---

## 🎯 Recommended Approach

### **Option 1: Background Worker** ✅

This is the **best approach** for the following reasons:

1. **Non-blocking**: Reads remain fast (6-7 seconds with aggressive timeout)
2. **Robust**: Can implement retry logic, error handling
3. **Efficient**: Deduplicates uploads, batches operations
4. **Production-ready**: Proven pattern (used by cache backend)
5. **Clean lifecycle**: Tied to Fs context, no leaks

---

## 📐 Implementation Plan

### Phase 1: Core Infrastructure

1. **Add upload queue to `Fs` struct**:
   ```go
   type Fs struct {
       // ... existing fields ...
       uploadQueue   *uploadQueue
       uploadWorkers int
       uploadCtx     context.Context
       uploadCancel  context.CancelFunc
   }
   ```

2. **Initialize in `NewFs`**:
   ```go
   f.uploadCtx, f.uploadCancel = context.WithCancel(context.Background())
   f.uploadQueue = newUploadQueue()
   
   // Start background workers
   for i := 0; i < f.uploadWorkers; i++ {
       go f.backgroundUploader(f.uploadCtx)
   }
   ```

3. **Cleanup** (add to `Fs` if not already present):
   ```go
   func (f *Fs) Shutdown() {
       f.uploadCancel()
       // Wait for workers to finish
   }
   ```

### Phase 2: Queue Management

1. **Implement `uploadQueue`**:
   - Map to track pending uploads (deduplicate)
   - Channel for job distribution
   - Mutex for thread safety

2. **Implement `queueParticleUpload()`**:
   - Check if already queued (deduplicate)
   - Add to queue
   - Log INFO message

### Phase 3: Integration

1. **Modify `Object.Open()`**:
   - After reconstruction, queue missing particle
   - Don't block on upload
   - Log self-healing action

2. **Implement `uploadParticle()`**:
   - Create particle data
   - Use `Put()` on appropriate backend
   - Handle errors gracefully
   - Log success/failure

### Phase 4: Configuration

1. **Add config options**:
   ```go
   type Options struct {
       // ... existing options ...
       SelfHealing      bool   `config:"self_healing"`
       UploadWorkers    int    `config:"upload_workers"`
       UploadRetries    int    `config:"upload_retries"`
   }
   ```

2. **Defaults**:
   - `self_healing = true` (enabled by default)
   - `upload_workers = 2` (2 concurrent uploads)
   - `upload_retries = 3` (retry failed uploads)

---

## ⚠️ Important Considerations

### 1. Context Management
- **Problem**: Request context may be cancelled after read completes
- **Solution**: Use `context.Background()` or `f.uploadCtx` for uploads
- **Tradeoff**: Upload continues even if user cancels operation

### 2. Error Handling
- **Problem**: User won't see upload errors (read already succeeded)
- **Solution**: 
  - Log errors at ERROR level
  - Implement retry logic (3 retries with exponential backoff)
  - Consider optional notification mechanism

### 3. Deduplication
- **Problem**: Multiple concurrent reads of same file
- **Solution**: Track in-progress uploads in map (key: remote+particleType)
- **Cleanup**: Remove from map after completion/failure

### 4. Memory Management
- **Problem**: Holding particle data in memory until upload completes
- **Solution**:
  - Limit queue size (e.g., 100 pending uploads)
  - Drop oldest if queue full (with WARNING log)
  - Consider spilling to temp files for large particles

### 5. Testing
- **Challenge**: Testing asynchronous background operations
- **Approach**:
  - Add synchronous mode for testing (`upload_workers = 0` = synchronous)
  - Add method to wait for queue to drain
  - Test with small files first

### 6. Monitoring
- **Metrics to track**:
  - Number of self-healing uploads queued
  - Number of successful uploads
  - Number of failed uploads
  - Queue depth

---

## 🚀 Benefits

1. **Automatic Recovery**: System self-heals without manual intervention
2. **No Performance Impact**: Reads remain fast (6-7s with aggressive timeout)
3. **Transparent**: User gets data immediately, healing happens in background
4. **Resilient**: Retries on failure, handles errors gracefully
5. **Production-Ready**: Based on proven patterns from cache backend

---

## 🎬 Next Steps

1. **Decision**: Confirm Option 1 (Background Worker) is acceptable
2. **Design Review**: Review implementation plan
3. **Prototype**: Implement Phase 1 (core infrastructure)
4. **Test**: Verify with local backends first
5. **Iterate**: Add queue management, integration
6. **Test**: Comprehensive testing with MinIO
7. **Document**: Update README.md with self-healing feature

---

## 📊 Comparison with Commercial Solutions

| Feature | level3 (proposed) | Ceph | ZFS | RAID Hardware |
|---------|-------------------|------|-----|---------------|
| Auto-reconstruction | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |
| Background healing | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |
| Non-blocking reads | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |
| Retry logic | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |
| Deduplication | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |

**Verdict**: The proposed implementation matches commercial RAID systems! 🎉

---

**Conclusion**: **Self-healing during reads is architecturally feasible** and can be implemented using the **background worker pattern** from the cache backend. This will provide automatic recovery without impacting read performance.

