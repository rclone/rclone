# List("") Behavior Analysis

## Key Questions Answered

### 1. Does union backend use List("")?

**Yes!** Union backend's `List()` function (union.go:684-713) calls `u.List(ctx, dir)` on each upstream backend. It does NOT recurse - it only lists the immediate directory.

**Error handling**:
- If ALL upstreams return `ErrorDirNotFound` → returns `ErrorDirNotFound` (line 707-708)
- Otherwise merges entries from all upstreams

### 2. Does List("") recurse?

**No!** `List()` does NOT recurse. It only lists the immediate directory contents.

- `List()` - Lists immediate directory only (non-recursive)
- `ListR()` - Lists recursively (if implemented)
- `ListP()` - Lists with pagination (if implemented)

**Our implementation**: `raid3.go:844-1012` - calls `f.even.List(ctx, dir)`, `f.odd.List(ctx, dir)`, `f.parity.List(ctx, dir)` - all non-recursive.

### 3. Does listing directories fail?

**Union backend behavior** (union.go:700-710):
- If ALL upstreams return `ErrorDirNotFound` → returns `ErrorDirNotFound`
- If ANY upstream succeeds → merges entries (even if some failed)

**Local backend behavior** (local.go:610-617):
- Calls `os.Stat(fsDirPath)` first
- If `os.Stat` fails → returns `fs.ErrorDirNotFound`
- If `os.Stat` succeeds → opens directory and lists contents

### 4. Should we use --max-depth 1?

**Not relevant for List()** - `List()` doesn't recurse by default.

`--max-depth` is only relevant for:
- `rclone ls` - recurses by default, use `--max-depth 1` to stop recursion
- `rclone lsl` - recurses by default, use `--max-depth 1` to stop recursion
- `rclone lsd` - doesn't recurse by default, use `-R` to recurse
- `rclone lsf` - doesn't recurse by default, use `-R` to recurse
- `rclone lsjson` - doesn't recurse by default, use `-R` to recurse

### 5. May 'rclone lsf' help?

**No** - `lsf` uses `List()` internally, so it would have the same behavior. It's just a different output format.

## What We Discovered

**Debug output from test**:
```
Rmdir: even.List("") returned nil (directory exists or is empty)
Rmdir: odd.List("") returned nil (directory exists or is empty)
Rmdir: parity.List("") returned nil (directory exists or is empty)
Rmdir: Directory "" exists on at least one backend, proceeding with removal
```

**Root cause identified**:
- `t.TempDir()` creates **empty directories that exist on the filesystem**
- When we call `List("")` on an existing but empty directory, it returns `(empty_list, nil)` - **no error**
- Our check sees `nil` and thinks the directory exists
- We proceed with `Rmdir`, which succeeds (removes empty directories)
- But the test expects an error because the directory "doesn't exist" from rclone's perspective

**The problem**: The test creates temp directories that **physically exist** (but are empty), so `List("")` returns no error. The test expects `Rmdir("")` to fail when the directory hasn't been explicitly created via `Mkdir()`, but that's not how rclone works - if the directory exists on the filesystem, `List("")` returns no error.

## Test Plan

1. Create a test that:
   - Creates temp directories (evenDir, oddDir, parityDir)
   - Creates Fs with root `rclone-test-xxxxx` (subdirectory that doesn't exist)
   - Calls `f.even.List(ctx, "")` directly
   - Checks what error it returns

2. Compare with union backend:
   - How does union backend handle this case?
   - Does it correctly return `ErrorDirNotFound`?

3. Check if there's something in our List() implementation that's interfering:
   - Auto-cleanup logic?
   - Auto-heal logic?
   - Error handling that might mask `ErrorDirNotFound`?
