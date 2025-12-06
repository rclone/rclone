# Lock/Verify: Rclone-Specific or Backend Feature?

**Date**: December 4, 2025  
**Question**: Is lock/verify a rclone-specific feature or base functionality provided by storage backends?

---

## 🎯 Short Answer

**Lock/verify is NOT a standard rclone feature, and most storage backends don't provide explicit locking mechanisms.**

Lock/verify would need to be **implemented by the level3 backend itself** as a custom coordination mechanism.

---

## 📋 What Rclone Provides

### Standard Backend Features

Rclone's `Features` interface (in `fs/features.go`) defines what backends can expose:

```go
type Features struct {
    // ... many feature flags ...
    
    // Move src to this remote using server-side move operations
    Move func(ctx context.Context, src Object, remote string) (Object, error)
    
    // Copy src to this remote using server-side copy operations  
    Copy func(ctx context.Context, src Object, remote string) (Object, error)
    
    // ... other operations ...
}
```

**Key Observations**:
- ✅ Backends can expose `Move()` operations
- ✅ Backends can expose `Copy()` operations
- ❌ **NO explicit `Lock()` method**
- ❌ **NO `Prepare()` or `Commit()` methods**
- ❌ **NO transaction support**

### What Backends Actually Provide

**Most backends support**:
- ✅ **Atomic rename/move operations** (when available)
  - Local filesystem: `os.Rename()` is atomic
  - S3: `CopyObject` + `DeleteObject` (not atomic, but server-side)
  
- ❌ **Explicit locking mechanisms**:
  - S3/MinIO: No native file locking
  - Most cloud storage: No locking
  - Local filesystem: File locking exists (`flock`), but rclone doesn't use it

---

## 🔍 Lock Mechanisms Found in Rclone

### 1. **Rclone-Specific Locking** (Not Backend Feature)

**Bisync lock files**:
```go
// cmd/bisync/lockfile.go
// Creates a .lck file to prevent concurrent bisync runs
// This is rclone-specific, not a backend feature
lockFile = basePath + ".lck"
os.WriteFile(lockFile, pidStr, permissions)
```

**SFTP string locking** (internal coordination):
```go
// backend/sftp/stringlock.go
// Internal lock for coordinating operations within SFTP backend
// Not exposed as a backend feature
```

**These are**:
- ✅ Rclone-specific coordination mechanisms
- ✅ Implemented at rclone level, not backend level
- ❌ NOT part of backend Features interface
- ❌ NOT provided by storage backends

---

## 🏗️ What Backends DO Provide

### 1. **Atomic Operations** (When Available)

**Local Filesystem**:
```go
// backend/local/local.go
// os.Rename() is atomic on most filesystems
err := os.Rename(src, dst)
```

**S3**:
```go
// backend/s3/s3.go
// Uses CopyObject + DeleteObject
// Not atomic, but server-side (faster than download/upload)
```

**Key Point**: Backends provide **atomic operations** (when available), but NOT explicit locking mechanisms.

### 2. **Server-Side Operations**

Backends can provide:
- ✅ Server-side copy (`Copy()`)
- ✅ Server-side move (`Move()`)
- ✅ Server-side delete

But NOT:
- ❌ Distributed locks
- ❌ Transaction support
- ❌ Prepare/commit phases

---

## 💡 For Level3: Lock/Verify Would Be Custom Implementation

### Option 1: **Rclone-Level Implementation** (Level3 Backend)

Lock/verify would be implemented **inside the level3 backend**:

```go
// backend/level3/level3.go

func (f *Fs) prepareMove(ctx context.Context, src, dst string) error {
    // Custom implementation using backend operations
    
    // "Lock" = Create marker files
    lockMarker := ".level3-move-lock-" + uuid.New()
    f.even.Put(ctx, bytes.NewReader([]byte{}), lockMarker)
    
    // "Verify" = Check all backends ready
    evenObj, err := f.even.NewObject(ctx, src)
    if err != nil {
        return err
    }
    // ... check odd, parity
    
    return nil
}
```

**This would**:
- ✅ Use standard backend operations (`Put()`, `NewObject()`, etc.)
- ✅ Work with any backend (no special backend support needed)
- ✅ Be implemented entirely in level3 backend
- ❌ NOT be a backend feature (not in Features interface)

### Option 2: **Use Backend's Atomic Operations**

Rely on backend's native atomic operations:

```go
// Most backends support atomic rename/move
// Use backend's Move() if it's atomic
if do := f.even.Features().Move; do != nil {
    // Backend provides atomic move
    return do(ctx, obj, remote)
}
```

**This would**:
- ✅ Use backend's native atomic operations
- ✅ No custom locking needed
- ❌ Only works if backend supports atomic operations
- ❌ Still need to coordinate across 3 backends

---

## 📊 Comparison Table

| Feature | Rclone Provides? | Backends Provide? | Level3 Needs? |
|---------|-----------------|-------------------|---------------|
| **Lock() method** | ❌ No | ❌ No (most) | ✅ Would need to implement |
| **Prepare() method** | ❌ No | ❌ No | ✅ Would need to implement |
| **Commit() method** | ❌ No | ❌ No | ✅ Would need to implement |
| **Atomic Move()** | ✅ Yes (via Features) | ✅ Yes (some backends) | ✅ Can use if available |
| **Atomic Rename()** | ✅ Yes (local) | ✅ Yes (local filesystem) | ✅ Can use if available |
| **Transaction support** | ❌ No | ❌ No | ✅ Would need to implement |
| **File locking** | ❌ Not exposed | ✅ Some (local) | ⚠️ Available but not used by rclone |

---

## 🔧 How Level3 Could Implement Lock/Verify

### Approach 1: **Marker Files** (Rclone-Level)

```go
func (f *Fs) prepareMove(ctx context.Context, src, dst string) error {
    // Create lock markers using standard Put() operations
    lockFile := ".level3-prepare-" + uuid.New()
    
    // "Lock" = Create marker files on all backends
    f.even.Put(ctx, bytes.NewReader([]byte("prepared")), lockFile)
    f.odd.Put(ctx, bytes.NewReader([]byte("prepared")), lockFile)
    f.parity.Put(ctx, bytes.NewReader([]byte("prepared")), lockFile)
    
    // "Verify" = Check all backends can access source
    // (using standard NewObject() calls)
    
    return nil
}
```

**Uses**: Standard backend operations (`Put()`, `NewObject()`)  
**Works with**: Any backend  
**Limitation**: Not real locking (concurrent operations could interfere)

### Approach 2: **Optimistic Locking**

```go
func (f *Fs) prepareMove(ctx context.Context, src, dst string) error {
    // No explicit locks - just verify readiness
    // Rely on atomic operations and fast failure
    
    // Verify all backends ready
    evenObj, errEven := f.even.NewObject(ctx, src)
    oddObj, errOdd := f.odd.NewObject(ctx, src)
    parityObj, errParity := f.parity.NewObject(ctx, src)
    
    if errEven != nil || errOdd != nil || errParity != nil {
        return fmt.Errorf("not all backends ready")
    }
    
    // If all verified, proceed with atomic moves
    return nil
}
```

**Uses**: Standard backend operations  
**Works with**: Any backend  
**No locks**: Just verification before move

### Approach 3: **Use Backend's Atomic Operations**

```go
// Rely on backend's atomic Move() operations
// If backend supports atomic move, use it
// Still need to coordinate across 3 backends
```

---

## ✅ Conclusion

**Lock/verify is NOT a standard rclone or backend feature**. For level3's two-phase commit:

1. **Would need to implement lock/verify in level3 backend**
   - Using standard backend operations (`Put()`, `NewObject()`, etc.)
   - Custom coordination mechanism
   - Not using backend-provided locking (because it doesn't exist)

2. **Most backends don't provide locking**
   - S3/MinIO: No locking
   - Cloud storage: Usually no locking
   - Local filesystem: Has locking (`flock`), but rclone doesn't expose it

3. **Backends DO provide atomic operations** (when available)
   - Can use backend's `Move()` if atomic
   - Still need to coordinate across multiple backends

4. **Implementation would be rclone-level** (in level3 backend)
   - Using standard operations
   - Custom coordination logic
   - Works with any backend type

**Bottom Line**: Lock/verify for two-phase commit would be a **custom implementation in the level3 backend**, not something provided by rclone or storage backends.


