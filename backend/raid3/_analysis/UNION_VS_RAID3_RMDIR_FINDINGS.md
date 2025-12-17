# Union Backend vs RAID3 Backend: Rmdir Findings

## Test Results

- ✅ **Union backend**: PASSES `TestStandard/FsRmdirNotFound`
- ❌ **RAID3 backend**: FAILS `TestStandard/FsRmdirNotFound`

## Key Discovery

**Debug output shows**:
```
even root: "/var/folders/.../001/rclone-test-juyucim3nati"
odd root: "/var/folders/.../002/rclone-test-juyucim3nati"
parity root: "/var/folders/.../003/rclone-test-juyucim3nati"

Rmdir: even.List("") returned nil (directory exists or is empty)
Rmdir: odd.List("") returned nil (directory exists or is empty)
Rmdir: parity.List("") returned nil (directory exists or is empty)
```

**The subdirectory `rclone-test-xxxxx` EXISTS on the filesystem!**

But the test calls `Rmdir("")` BEFORE `Mkdir("")`, so it shouldn't exist. This suggests:
1. The subdirectory is being created somewhere before the test runs
2. Or `List("")` is not behaving as expected for non-existent directories

## How Union Backend Works

### Rmdir Flow

1. **Calls `f.action(ctx, dir)`** (union.go:128)
   - Uses action policy (e.g., `EpAll`)
   - Calls `epall()` which uses `findEntry()` to check existence

2. **`epall()` checks existence** (epall.go:24-49)
   - For each upstream: `findEntry(ctx, rfs, remote)` where `remote = path.Join(u.RootPath, filePath)`
   - If `findEntry` returns `nil` for all upstreams → returns `fs.ErrorObjectNotFound`
   - If any `findEntry` returns non-nil → includes that upstream in results

3. **`findEntry()` for root directory** (policy.go:103-126)
   - For root (`remote == dir == ""`): calls `f.List(ctx, dir)` where `dir = ""`
   - If `List("")` returns **error** → returns `nil` (doesn't exist)
   - If `List("")` returns **no error** → returns `fs.NewDir("", time.Time{})` (exists)

4. **If `action()` returns error** (union.go:128-135)
   - Returns that error (e.g., `fs.ErrorObjectNotFound`)
   - **This is why union backend passes the test!**

## How RAID3 Backend Works (Current)

### Rmdir Flow

1. **Calls `f.checkAllBackendsAvailable(ctx)`** (raid3.go:1228)
   - Health check - ensures all 3 backends are available

2. **Checks existence** (raid3.go:1243-1263)
   - Calls `f.even.List(ctx, dir)` directly
   - If `List("")` returns `nil` → directory exists
   - If `List("")` returns `ErrorDirNotFound` → directory doesn't exist
   - Same for odd and parity

3. **If all return `ErrorDirNotFound`** (raid3.go:1267-1268)
   - Returns `fs.ErrorDirNotFound`
   - **This should work, but it's not!**

## The Problem

**Our debug shows**:
- `f.even.List(ctx, "")` returns `nil` (no error)
- This means the directory exists (or `List()` is not detecting non-existent directories correctly)

**But the test expects**:
- `Rmdir("")` to return an error when the directory doesn't exist
- The directory shouldn't exist because `Mkdir("")` hasn't been called yet

## Possible Causes

1. **Subdirectory is being created somewhere**
   - Maybe `cache.Get()` creates it? (Unlikely - cache doesn't create directories)
   - Maybe `NewFs()` creates it? (Unlikely - local backend's `NewFs` doesn't create directories)
   - Maybe test setup creates it? (Need to check)

2. **`List("")` behavior is different than expected**
   - Maybe local backend's `List("")` returns `nil` for non-existent directories?
   - But local backend code shows it should return `ErrorDirNotFound`

3. **Union backend uses different approach**
   - Union backend calls `findEntry()` which calls `List()` on the **parent directory**
   - For root, it calls `List("")` on the upstream's RootFs
   - But wait - union backend's upstreams also point to subdirectories, so it should have the same issue!

## Next Steps

1. **Check if union backend's upstreams also point to subdirectories**
   - If yes, how does it handle `List("")` on non-existent subdirectory?
   - Does it return `ErrorDirNotFound` correctly?

2. **Verify what `f.even.List(ctx, "")` actually does**
   - Add more detailed logging to see what `os.Stat()` returns
   - Check if the subdirectory actually exists on the filesystem

3. **Compare with union backend's `findEntry()` approach**
   - Maybe we need to use a similar pattern?
   - But union backend also calls `List("")` directly, so it should have the same issue

## Key Difference Discovered!

**Union backend uses a different pattern**:

1. **Union backend's `epall()`** (epall.go:31-33):
   ```go
   rfs := u.RootFs  // Local backend pointing to temp directory (e.g., /tmp/001)
   remote := path.Join(u.RootPath, filePath)  // "rclone-test-xxxxx" + "" = "rclone-test-xxxxx"
   if findEntry(ctx, rfs, remote) != nil {
   ```

2. **`findEntry()` for non-root** (policy.go:103-125):
   ```go
   dir := parentDir(remote)  // parentDir("rclone-test-xxxxx") = ""
   entries, err := f.List(ctx, dir)  // List("") on temp directory (exists!)
   // Then searches for "rclone-test-xxxxx" in entries
   // If not found, returns nil → epall returns ErrorObjectNotFound
   ```

**Key insight**: Union backend lists the **parent directory** (temp dir) which exists, then searches for the subdirectory in the entries. If not found, it correctly returns `ErrorObjectNotFound`.

**RAID3 backend**:
- `f.even` points directly to the subdirectory (`/tmp/001/rclone-test-xxxxx`)
- Calls `List("")` on the subdirectory itself
- If subdirectory doesn't exist, should return `ErrorDirNotFound`
- But it's returning `nil`!

**The mystery**: Why does `List("")` on a non-existent subdirectory return `nil` instead of `ErrorDirNotFound`?

## Solution: Use Union Backend's Pattern

We should follow union backend's approach:
1. List the **parent directory** (temp dir)
2. Search for the subdirectory in the entries
3. If not found, return `ErrorDirNotFound`

This avoids the issue of `List("")` on non-existent directories.
