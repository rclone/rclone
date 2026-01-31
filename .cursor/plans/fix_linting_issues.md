# Plan: Fix Linting Issues for raid3 Backend

## Overview
Fix 112 linting issues found by golangci-lint in the raid3 backend to meet rclone's code quality standards.

## Issue Categories

### 1. errcheck (90 issues) - HIGH PRIORITY
**Problem**: Error return values from `Close()`, `os.Chmod()`, `os.RemoveAll()`, and `Shutdown()` are not checked.

**Solution Pattern**:
- For `defer Close()` in functions with named error returns: Use `fs.CheckClose(reader, &err)`
- For `defer Close()` without error context: Use `_ = reader.Close()` (explicit ignore)
- For test cleanup: Use `_ = os.Chmod()` and `_ = os.RemoveAll()` (test cleanup errors are acceptable to ignore)

**Files to fix**:
- `backend/raid3/object.go` (18 instances)
- `backend/raid3/operations.go` (6 instances)
- `backend/raid3/particles.go` (4 instances)
- `backend/raid3/raid3_test.go` (30+ instances)
- `backend/raid3/raid3_errors_test.go` (20+ instances)
- `backend/raid3/raid3_heal_test.go` (4 instances)

### 2. gocritic (2 issues) - MEDIUM PRIORITY
**Problem**: Using `log.Printf` instead of `fs.Logf` in `streammerger.go`

**Solution**: Replace `log.Printf` with `fs.Logf(nil, ...)` or pass appropriate fs.Fs context

**Files to fix**:
- `backend/raid3/streammerger.go` (lines 176, 223)

### 3. ineffassign (3 issues) - LOW PRIORITY
**Problem**: Ineffectual assignments that don't affect program behavior

**Solution**: Remove or fix the assignments

**Files to fix**:
- `backend/raid3/commands.go` (line 53: `opt = map[string]string{}`)
- `backend/raid3/object.go` (lines 1703, 1746: `dirName = d.remote`)

### 4. revive (10 issues) - MEDIUM PRIORITY

#### 4a. Missing comments (3 issues)
**Problem**: Exported methods lack comments

**Solution**: Add godoc comments

**Files to fix**:
- `backend/raid3/metadata.go`: `MkdirMetadata`, `DirSetModTime`
- `backend/raid3/operations.go`: `Move`

#### 4b. indent-error-flow (4 issues)
**Problem**: Unnecessary `else` blocks after `return` statements

**Solution**: Remove `else` and outdent the block

**Files to fix**:
- `backend/raid3/object.go` (lines 605, 801, 924)
- `backend/raid3/particles.go` (line 309)

#### 4c. context-as-argument (3 issues)
**Problem**: `context.Context` should be first parameter

**Solution**: Reorder function parameters

**Files to fix**:
- `backend/raid3/raid3_heal_command_test.go` (line 19)
- `backend/raid3/raid3_rebuild_test.go` (line 45)
- `backend/raid3/raid3_test.go` (line 2620)

### 5. staticcheck (3 issues) - MEDIUM PRIORITY

#### 5a. Deprecated strings.Title (1 issue)
**Problem**: `strings.Title` is deprecated

**Solution**: Use `golang.org/x/text/cases` instead

**File to fix**:
- `backend/raid3/commands.go` (line 138)

#### 5b. Unnecessary nil check (2 issues)
**Problem**: `len()` for nil slices is defined as zero

**Solution**: Remove nil check before `len()`

**Files to fix**:
- `backend/raid3/particles.go` (line 564)
- `backend/raid3/streammerger.go` (line 54)

### 6. unused (4 issues) - LOW PRIORITY
**Problem**: Unused constants and functions

**Solution**: Remove unused code

**Files to fix**:
- `backend/raid3/constants.go`: `minChunkSize` (line 35)
- `backend/raid3/helpers.go`: `validateChunkSize` (line 443)
- `backend/raid3/raid3.go`: `checkSubdirectoryExists` (line 1335)
- `backend/raid3/raid3_test.go`: `remotefname` (line 237)

## Implementation Strategy

### Phase 1: Fix errcheck issues (90 issues)
1. **Production code** (`object.go`, `operations.go`, `particles.go`):
   - For functions with named error returns: Use `fs.CheckClose(reader, &err)`
   - For functions without error context: Use `_ = reader.Close()`

2. **Test code** (all `*_test.go` files):
   - Use `_ = rc.Close()` for cleanup
   - Use `_ = os.Chmod()` and `_ = os.RemoveAll()` for test cleanup

### Phase 2: Fix other issues (22 issues)
1. Fix gocritic (2 issues)
2. Fix revive issues (10 issues)
3. Fix staticcheck (3 issues)
4. Fix ineffassign (3 issues)
5. Remove unused code (4 issues)

### Phase 3: Verification
1. Run `golangci-lint run ./backend/raid3/...` - should pass
2. Run `go test ./backend/raid3 -v` - all tests should pass
3. Verify no regressions

## Notes

- `fs.CheckClose` is the rclone pattern for handling Close() errors in defer statements when there's an error return value
- For test cleanup, explicitly ignoring errors with `_ =` is acceptable
- Some unused code might be kept for future use - verify before removing
- Context parameter ordering follows Go conventions (context should be first)
