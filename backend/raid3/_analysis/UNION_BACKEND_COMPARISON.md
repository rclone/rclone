# Union Backend vs RAID3 Backend: Rmdir Comparison

## Test Results

- ✅ **Union backend**: PASSES `TestStandard/FsRmdirNotFound`
- ❌ **RAID3 backend**: FAILS `TestStandard/FsRmdirNotFound`

## How Union Backend Works

### Test Setup
- Uses `union.MakeTestDirs(t, 3)` which creates 3 temp directories
- Uses `RandomRemoteName` which creates subdirectory path `rclone-test-xxxxx`
- Union backend's upstreams point to temp directories with root `rclone-test-xxxxx`

### Rmdir Flow (union.go:127-144)

```go
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
    upstreams, err := f.action(ctx, dir)  // ← Checks existence FIRST
    if err != nil {
        // If none of the backends can have empty directories then
        // don't complain about directories not being found
        if !f.features.CanHaveEmptyDirectories && err == fs.ErrorObjectNotFound {
            return nil
        }
        return err  // ← Returns error if directory doesn't exist
    }
    // ... proceed with Rmdir on upstreams
}
```

### Action Policy (EpAll) Flow (epall.go:24-49)

```go
func (p *EpAll) epall(ctx context.Context, upstreams []*upstream.Fs, filePath string) ([]*upstream.Fs, error) {
    // For each upstream:
    rfs := u.RootFs  // The underlying Fs (e.g., local backend)
    remote := path.Join(u.RootPath, filePath)  // Join root path with filePath
    if findEntry(ctx, rfs, remote) != nil {  // ← Check if entry exists
        ufs[i] = u
    }
    // ...
    if len(results) == 0 {
        return nil, fs.ErrorObjectNotFound  // ← Returns ErrorObjectNotFound if none found
    }
}
```

### findEntry() for Root Directory (policy.go:103-126)

```go
func findEntry(ctx context.Context, f fs.Fs, remote string) fs.DirEntry {
    remote = clean(remote)
    dir := parentDir(remote)
    entries, err := f.List(ctx, dir)  // ← Calls List() on parent directory
    if remote == dir {  // ← For root directory (remote == dir == "")
        if err != nil {
            return nil  // ← Directory doesn't exist
        }
        return fs.NewDir("", time.Time{})  // ← Directory exists
    }
    // ... search in entries
}
```

**Key Insight**: For root directory (`remote == dir == ""`):
- If `List("")` returns an **error** → `findEntry` returns `nil` → directory doesn't exist
- If `List("")` returns **no error** (even empty list) → `findEntry` returns directory entry → directory exists

## How RAID3 Backend Works (Current Implementation)

### Test Setup
- Uses `t.TempDir()` to create 3 temp directories
- Uses `RandomRemoteName` which creates subdirectory path `rclone-test-xxxxx`
- RAID3 backend's upstreams point to temp directories with root `rclone-test-xxxxx`

### Rmdir Flow (raid3.go:1227-1268)

```go
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
    // Pre-flight check: Enforce strict RAID 3 delete policy
    if err := f.checkAllBackendsAvailable(ctx); err != nil {
        return fmt.Errorf("rmdir blocked in degraded mode (RAID 3 policy): %w", err)
    }

    // Check if directory exists before attempting removal
    dirExists := false
    _, evenListErr := f.even.List(ctx, dir)  // ← Direct List() call
    if evenListErr == nil {
        dirExists = true
    } else if !errors.Is(evenListErr, fs.ErrorDirNotFound) {
        dirExists = true
    }
    // ... same for odd and parity
    
    if !dirExists {
        return fs.ErrorDirNotFound
    }
    // ... proceed with Rmdir
}
```

## The Problem

**Debug output shows**:
```
Rmdir: even.List("") returned nil (directory exists or is empty)
Rmdir: odd.List("") returned nil (directory exists or is empty)
Rmdir: parity.List("") returned nil (directory exists or is empty)
```

**Why this happens**:
- `t.TempDir()` creates **empty directories that exist on the filesystem**
- When we call `f.even.List(ctx, "")` on `evenDir/rclone-test-xxxxx` (which doesn't exist), the local backend's `List()` does:
  - `os.Stat("evenDir/rclone-test-xxxxx")` → returns `os.ErrNotExist`
  - Should return `fs.ErrorDirNotFound`
- **But**: The subdirectory `rclone-test-xxxxx` doesn't exist, so `List("")` should return `ErrorDirNotFound`

**Wait**: Let me check what `f.even` actually points to. If `f.even` is already pointing to `evenDir/rclone-test-xxxxx`, then `List("")` would list that subdirectory, which doesn't exist, so it should return `ErrorDirNotFound`.

But our debug shows it returns `nil`! This means the subdirectory **does exist** somehow, or `List("")` is not behaving as expected.

## Key Difference

**Union backend**:
- Uses `findEntry()` which calls `List()` on the **parent directory** to check if the target exists
- For root, it checks if `List("")` returns an error
- If all upstreams return `ErrorObjectNotFound` from `epall()`, `Rmdir` returns that error

**RAID3 backend**:
- Directly calls `List("")` on each backend
- Checks if any return `ErrorDirNotFound`
- But `List("")` is returning `nil` (no error) even though the directory doesn't exist

## Next Steps

1. **Verify what `f.even.List(ctx, "")` actually does** - Add more detailed logging
2. **Check if the subdirectory exists** - Maybe it's being created somewhere?
3. **Compare with union backend's `findEntry()` approach** - Should we use a similar pattern?
