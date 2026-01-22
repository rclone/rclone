# Open Questions - raid3 Backend

This document tracks open design questions and pending decisions for the raid3 backend, serving as a question registry (centralized list of issues requiring decisions), priority tracking (high/medium/low priority classification), status monitoring (active, resolved, or deferred questions), and decision workflow (process for moving questions to decisions). Process: Add questions as they arise, document decisions in [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md) when resolved. Last Updated: December 8, 2025. For resolved decisions, see [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md). For user documentation, see [`README.md`](../README.md).

---

## 🔴 High Priority

---

### Q24: Intermittent FsListRLevel2 Test Failure (Duplicate Directory)
**Status**: 🔴 **ACTIVE** - Investigation ongoing  
**Priority**: High (test reliability)

**Issue**: The `FsListRLevel2` test intermittently fails (~50% failure rate) with duplicate directory entries:
- **Expected**: `[]string{"hello? sausage", "hello? sausage/êé"}` (2 entries)
- **Actual**: `[]string{"hello? sausage", "hello? sausage/êé", "hello? sausage/êé"}` (3 entries - duplicate)

**Investigation Status**:
- ✅ Our `ListR` callback correctly returns unique entries (verified with duplicate detection code - no duplicates before callback)
- ✅ Removed `stats.ResetErrors()` to fix test state leakage (this fixed other intermittent failures)
- ✅ Deduplication logic in `ListR` is correct (uses map with remote path as key)
- ✅ Confirmed duplicate appears **after** our callback returns (in `walkRDirTree`/`DirTree` processing)
- 🔍 Test passes individually but fails intermittently when run together (~50% failure rate)
- 🔍 Union backend doesn't have this issue (verified - union tests pass consistently)

**Technical Analysis**:
- `walkRDirTree` processes directories differently based on slash count:
  - `"hello? sausage"` (0 slashes, maxLevel=2): uses `dirs.AddDir(x)` (slashes < maxLevel-1)
  - `"hello? sausage/êé"` (1 slash, maxLevel=2): uses `dirs.Add(x)` (slashes == maxLevel-1)
- `DirTree.Add()` doesn't deduplicate - it just appends to slices
- `walkR()` extracts entries from `DirTree` and passes them to test callback
- Our callback is called exactly once (verified)

**Possible Causes**:
1. Framework bug in `walkRDirTree`/`DirTree` when `maxLevel >= 0` - most likely cause
2. Race condition or state issue in `DirTree` processing (intermittent nature suggests this)
3. Difference in how `raid3` vs `union` backend entries are processed by framework

**Next Steps**:
- Continue investigation to create minimal reproducer
- If confirmed framework bug, file upstream issue with rclone project
- Document as known limitation if confirmed framework bug
- **Temporary workaround**: Test skipped in `test_runner.sh` (see test runner for details)

**References**:
- Investigation notes: `backend/raid3/_analysis/DUPLICATE_DIRECTORY_BUG_ANALYSIS.md`
- Code: `backend/raid3/list.go` (ListR implementation)
- Framework code: `fs/walk/walk.go` (walkRDirTree), `fs/dirtree/dirtree.go` (DirTree)

---

### Q14: Optimize Health Checks (Add Caching)
**Status**: 🔴 **ACTIVE** - Performance optimization  
**Priority**: High (affects write performance)

Health checks run before every write operation, causing network I/O overhead. Add TTL-based caching to reduce redundant checks and improve write performance.

---

### Q15: Make Background Worker Context Respect Cancellation
**Status**: 🔴 **ACTIVE** - Resource management  
**Priority**: High (affects graceful shutdown)

Background upload workers use `context.Background()` and don't respect parent context cancellation. Derive worker context from parent context passed to `NewFs` or provide cancellation mechanism for graceful shutdown.

---

## 🟡 Medium Priority

### Q1: Update Rollback Not Working Properly
**Status**: 🚨 **ACTIVE** - Still needs implementation  
**Priority**: Medium

Update operation rollback not working properly when `rollback=true`. Put and Move rollback work correctly. Fix `updateWithRollback()` and `rollbackUpdate()` to correctly handle Copy+Delete fallback for backends without Move support. Add comprehensive `update-fail` tests.

---

### Q10: Backend-Specific Commands Support 🤝 **COMMUNITY DISCUSSION**
**Status**: 🟡 **ACTIVE** - Awaiting rclone community discussion  
**Priority**: Medium-High

Should raid3 support backend-specific commands when all three remotes use the same backend type? S3 object tags are CRITICAL (lifecycle/billing/access control) - tags must be consistent across all three particles. Recommended: Support subset of commands (`set`, `settags`, `gettags`, `deletetags`, `get`, `cleanup`, `versioning`). Awaiting rclone community discussion before implementation.

---

### Q11: Bucket/Directory Renaming (DirMove) Limitation with S3
**Status**: 🟡 **ACTIVE** - Known limitation  
**Priority**: Low-Medium

S3/MinIO backends don't support bucket renaming (fundamental S3 API limitation). raid3's `DirMove` only works if all three underlying backends support it. Bucket renaming not supported; directory renaming works with DirMove-capable backends. Workaround: `rclone copy source dest` + `rclone purge source`. Consider improving error messages to clarify this is an S3 limitation.

---

### Q16: Make Hardcoded Values Configurable
**Status**: 🟡 **ACTIVE** - Code quality improvement  
**Priority**: Medium

Several hardcoded values (upload workers: 2, queue buffer: 100, shutdown timeout: 60s) cannot be tuned by users. Add configuration options for upload workers and queue buffer size to optimize for different workloads.

---

### Q21: Optimize Range Reads for Streaming
**Status**: 🟡 **ACTIVE** - Streaming optimization  
**Priority**: Medium

**Question**: Should range reads apply byte ranges directly to particle readers instead of reading entire particles and filtering?

**Context**: Currently, when reading a byte range (e.g., bytes 1000-2000 of a file), the implementation reads entire even and odd particles and then filters the output. This wastes bandwidth and I/O for partial reads, which is common in HTTP range requests, video streaming, and partial file access.

**Current Implementation** (`object.go:494-500`):
- Reads entire particles: `evenObj.Open(ctx, filteredOptions...)` (reads full particle)
- Filters output: `newRangeFilterReader(merger, rangeStart, rangeEnd, ...)`
- For a 1KB range read from a 1GB file, still reads ~500MB of particle data

**Proposed Optimization**:
- Calculate which byte ranges are needed from each particle based on the requested range
- Apply range options directly to particle readers: `evenObj.Open(ctx, &fs.RangeOption{Start: particleStart, End: particleEnd}, ...)`
- Only read the needed bytes from each particle

**Benefit**:
- Significantly reduces I/O for partial reads (e.g., 1KB range from 1GB file: ~500MB → ~1KB)
- Improves latency for range requests
- Better bandwidth utilization
- Common use case: HTTP range requests, video streaming, database partial reads

**Implementation Complexity**: Medium (3-5 days)
- Requires byte-to-particle coordinate mapping
- Need to handle odd-length files correctly
- Must support both RangeOption and SeekOption

**References**: 
- Current implementation: `backend/raid3/object.go:494-500`
- Performance analysis: `_analysis/performance/PERFORMANCE_ANALYSIS.md:265-270`

---

### Q25: Usage/Quota Caching with Aggregation
**Status**: 🟡 **ACTIVE** - Performance optimization  
**Priority**: Medium

**Question**: Should raid3 implement cached quota information with TTL-based expiration similar to union backend?

**Context**: Currently, `About()` method queries all three backends every time it's called, causing network I/O overhead and latency. The union backend implements sophisticated quota caching with background updates that could be adapted for raid3's aggregated usage reporting.

**Current Implementation** (`raid3.go:775-833`):
- Queries all 3 backends synchronously on every `About()` call
- No caching of quota information
- Each call causes 3 backend queries (even, odd, parity)
- Higher latency for monitoring/status commands

**Proposed Implementation** (inspired by union backend):
- Add cached `fs.Usage` struct with TTL expiration
- Background cache updates to avoid blocking
- Configurable cache duration (default: 120 seconds)
- Atomic cache expiry tracking with RWMutex protection

**Benefits**:
- Faster `About()` calls (uses cached values when available)
- Reduced backend load (fewer quota queries)
- Better performance for monitoring/status tools
- More efficient for frequently accessed quota information

**Technical Details**:
- Similar to union's `upstream/upstream.go` caching mechanism
- Cache structure: `usage *fs.Usage`, `cacheTime time.Duration`, `cacheExpiry atomic.Int64`
- Lazy cache updates: first call synchronous, subsequent calls trigger background refresh
- Handles backend errors gracefully (cached value on error)

**Implementation Complexity**: Medium (2-3 days)
- Add cache fields to `Fs` struct
- Implement cache update logic with background goroutines
- Add config option: `cache_time` (default: 120 seconds)
- Ensure thread-safe cache access with mutexes

**References**:
- Current implementation: `backend/raid3/raid3.go:775-833` (About method)
- Union backend reference: `backend/union/upstream/upstream.go:391-500` (caching mechanism)
- Related optimization: Q14 (Health Check Caching) uses similar TTL pattern

---

### Q12: Post-Rename Verification Checklist ⚠️ **VERIFY LATER**
**Status**: 🟡 **ACTIVE** - Items to verify after level3 → raid3 rename  
**Priority**: Medium

After rename from `level3` to `raid3`, verify: CI/CD configuration (`.github/workflows/*.yml`), external documentation (rclone wiki/docs), example configurations, code comments, variable/constant names. High priority: verify CI/CD and external docs. Most items already completed.

---

## 🟢 Low Priority

### Q3: Chunk/Block-Level Striping
**Status**: 🟢 **ACTIVE** - Low priority  
**Question**: Should raid3 support block-level striping instead of byte-level?

Current implementation uses byte-level (RAID 3 style). Block-level (RAID 5 style) would have fewer API calls but more complex implementation. Recommendation: Stay with byte-level (simpler, true RAID 3).

---

### Q6: Backend Help Command Behavior
**Status**: 🟢 **ACTIVE** - Low priority  
**Question**: How should `rclone backend help raid3:` behave?

Options: aggregated (like union), per-remote (like combine), or raid3-specific custom help. Recommendation: Start with raid3-specific custom help.

---

### Q8: Cross-Backend Move/Copy ⚠️ **NEEDS INVESTIGATION**
**Status**: 🟢 **ACTIVE** - Needs testing/investigation  
**Question**: How should raid3 handle copying FROM raid3 TO raid3?

Same backend overlap issue as `union` and `combine`. Likely fails with "overlapping remotes" error. Test this scenario.

---

### Q9: Compression Support with Streaming 🔮 **DECISION NEEDED**
**Status**: 🟢 **ACTIVE** - Research complete, awaiting decision  
**Question**: Should raid3 support optional compression (Snappy/LZ4) to reduce storage overhead?

**Context**: Current storage overhead is 150% (even + odd + parity). Compression could reduce this significantly (e.g., ~75% overhead with Snappy for text files, ~50% savings). **Critical**: Must compress BEFORE splitting to preserve patterns and achieve good compression ratio (compressing after splitting destroys patterns and reduces ratio by ~40%). Streaming support is now implemented (see Q2 below - resolved). Options: Snappy (fast, low CPU), LZ4 (very fast, low CPU), or configurable. Decision needed: whether to implement, which algorithm, and configuration approach.

---

### Q17: Improve Test Context Usage
**Status**: 🟢 **ACTIVE** - Test quality improvement  
**Priority**: Low

Many tests use `context.Background()` (53 instances found). Add timeouts to long-running tests using `context.WithTimeout()` for cancellation protection.

---

### Q18: Document Size() Context Limitation
**Status**: 🟢 **ACTIVE** - Documentation improvement  
**Priority**: Low

`Size()` method doesn't accept context parameter (matches rclone interface), internal operations use `context.Background()` which can't be cancelled. Document this limitation in code comments and README if needed.

---

### Q19: Add More Granular Error Types
**Status**: 🟢 **ACTIVE** - Error handling improvement  
**Priority**: Low

Current error handling uses generic `fmt.Errorf()`. Consider adding specific error types for common scenarios (degraded mode, particle missing, etc.) for better error classification and debugging.

---

### Q22: Parallel Reader Opening for Streaming Reads
**Status**: 🟢 **ACTIVE** - Streaming optimization  
**Priority**: Low

**Question**: Should particle readers be opened concurrently instead of sequentially?

**Context**: In `openStreaming()` (`object.go:479-488`), object lookup is already parallel (lines 434-442), but reader opening is sequential. This adds small latency overhead.

**Current Implementation**:
```go
evenReader, err := evenObj.Open(ctx, filteredOptions...)  // Sequential
oddReader, err := oddObj.Open(ctx, filteredOptions...)   // Then this
```

**Proposed Optimization**:
- Open both readers concurrently using `errgroup`
- Small latency improvement (typically <10ms per read operation)

**Benefit**: Minor latency improvement for read operations

**Implementation Complexity**: Low (1-2 hours)
- Simple change to use errgroup for parallel opening
- Already have pattern from other concurrent operations

**References**: 
- Current implementation: `backend/raid3/object.go:479-488`
- Similar pattern: `backend/raid3/raid3.go:1252-1293` (concurrent uploads)

---

### Q23: Improve StreamReconstructor Size Mismatch Handling
**Status**: 🟢 **ACTIVE** - Streaming optimization  
**Priority**: Low

**Question**: Should StreamReconstructor better handle size mismatches during streaming?

**Context**: When data and parity streams read different amounts during streaming reconstruction, the current implementation processes the minimum. A comment in the code mentions "future enhancement" for better buffering of excess data.

**Current Behavior**: Works correctly but processes minimum size, potentially requiring additional reads

**Proposed Enhancement**: Better buffering strategy for size mismatches to reduce number of read operations

**Benefit**: Minor efficiency improvement for degraded mode reads

**Implementation Complexity**: Low-Medium (1-2 days)
- Requires careful handling of buffering logic
- Must maintain correctness for reconstruction

**References**: 
- Current implementation: `backend/raid3/particles.go` (StreamReconstructor)
- Note: StreamMerger has been moved to `backend/raid3/streammerger.go`

---

## ✅ Resolved Questions

**Note**: These questions have been resolved and should be moved to [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md) for historical reference.

### Q2: Streaming Support for Large Files ✅ **RESOLVED**
**Date**: 2025-12-22  
**Status**: ✅ **RESOLVED** - Pipelined chunked streaming implemented

**Original Question**: Should raid3 support streaming for large files instead of loading entire file into memory?

**Resolution**: 
Implemented pipelined chunked streaming approach (see DD-009 in [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md)). The implementation reads files in 2MB chunks, splits each chunk into even/odd/parity particles, and uploads them sequentially while reading the next chunk in parallel. This provides bounded memory usage (~5MB) and enables efficient handling of very large files.

**Implementation Details**:
- Default mode: `use_streaming=true` (pipelined chunked approach)
- Legacy mode: `use_streaming=false` (buffered, loads entire file)
- Memory usage: ~5MB for double buffering (vs ~3× file size for buffered mode)
- Chunk size: 2MB read chunks (produces ~1MB per particle)
- All Go tests passing

**References**: 
- Design Decision: [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md) DD-009
- Implementation Analysis: `_analysis/SIMPLIFIED_PIPELINED_APPROACH.md`
- Refactoring Analysis: `_analysis/REVERT_VS_MODIFY_ANALYSIS.md`

### Q20: FsRmdirNotFound Test Failure ✅ **RESOLVED**
**Date**: 2025-12-18  
**Status**: ✅ **RESOLVED** - Test now passes

**Original Question**: `TestStandard/FsRmdirNotFound` test failing: `Rmdir("")` returns `nil` instead of `fs.ErrorDirNotFound` for non-existent root.

**Resolution**: 
Fixed by implementing existence check before attempting removal, following union backend pattern. The issue was that the health check (`checkAllBackendsAvailable()`) was called before the existence check, causing side effects. 

**Implementation**:
- Added `checkDirectoryExists()` helper function to check directory existence across all backends using `List()` calls
- Modified `Rmdir()` to check existence first (before health check) and return `fs.ErrorDirNotFound` immediately if directory doesn't exist
- Simplified error handling logic since existence is now checked upfront

**Test Results**: All `TestStandard/FsRmdirNotFound` tests now pass (standard, balanced, aggressive timeout modes).

**References**: 
- Implementation: `backend/raid3/raid3.go` (lines 743-828, 1315-1333)
- Union backend reference: `backend/union/union.go:127-144`

### Q4: Rebuild Command for Backend Replacement ✅ **IMPLEMENTED**
**Date**: 2025-11-02  
**Resolution Date**: 2025-12-07  
**Status**: ✅ **IMPLEMENTED** - The rebuild command is fully functional

**Original Question**: How should we implement RAID 3 rebuild when a backend is permanently replaced?

**Resolution**: 
The rebuild command has been fully implemented in `raid3.go` (function `rebuildCommand` starting at line 1230). All proposed features are working:

✅ **Implemented Features**: Manual rebuild command (`rclone backend rebuild raid3: [even|odd|parity]`), auto-detection (`rclone backend rebuild raid3:` auto-detects which backend needs rebuild), check-only mode (`-o check-only=true`), dry-run mode (`-o dry-run=true`), priority options (`-o priority=auto|dirs-small|dirs|small`).

**Documentation**: See `rclone backend help raid3:` for full usage details. Also documented in `README.md` section "Backend Commands > Rebuild Command".

---

### Q5: Configurable Write Policy ✅ **RESOLVED - DECISION MADE**
**Status**: ✅ **RESOLVED** - Decision: Not implementing (keep simple)

**Original Question**: Should users be able to choose degraded write mode?

**Resolution**: Not implementing for now. Current strict write policy (all 3 backends required) matches hardware RAID 3 behavior and ensures data consistency. Keep implementation simple.

**Reconsider if**: Users request this feature

**References**: `docs/ERROR_HANDLING.md` (discusses configurable write policy option)

---

### Q7: Move with Degraded Source ✅ **RESOLVED - DECISION MADE**
**Status**: ✅ **RESOLVED** - Decision: Keep current behavior (documented)

**Original Question**: Current behavior allows moving files with missing particles. Is this desired?

**Resolution**: Keep current behavior (flexible). Move succeeds even with degraded source, propagating degraded state to new location. This matches user expectations and avoids blocking moves unnecessarily.

**Documented**: This behavior is documented as known/expected.

**Reconsider if**: Users report confusion or data loss

---

## 📋 Process for Resolving Questions

### When a Question is Answered

Document the decision in [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md), update this file (move question to "Resolved" section or delete), implement the decision in code, update user documentation if user-facing, and add tests if needed.

### Template for New Questions:

```markdown
### Q#: [Title]
**Question**: [Clear question]

**Context**: [Why is this question important?]

**Options**:
- A) [Option 1]
- B) [Option 2]

**Investigation**: [What needs to be researched?]

**Recommendation**: [Your thoughts]
```

---

## 🎯 Quick Add Template

**Copy this when you have a new question**:

```markdown
### Q#: [SHORT_TITLE]
**Question**: How should [FEATURE] behave when [SCENARIO]?

**Context**: [Why this matters]

**Options**:
- A) [Approach 1] - [pros/cons]
- B) [Approach 2] - [pros/cons]

**Investigation**: 
- [ ] Check how [similar feature] works
- [ ] Test [scenario]

**Recommendation**: [Your initial thoughts]

**Priority**: High | Medium | Low
```

---

## 📊 Statistics

Total active questions: 18. Resolved questions: 5 (Q2, Q4, Q5, Q7, Q20). Active questions by priority: High Priority (3) - Q14: Health Check Caching, Q15: Background Worker Context, Q24: Intermittent FsListRLevel2 Test Failure. Medium Priority (7) - Q1: Update Rollback, Q10: Backend Commands, Q11: DirMove Limitation, Q12: Post-Rename Verification, Q16: Configurable Values, Q21: Range Read Optimization, Q25: Usage/Quota Caching. Low Priority (8) - Q3: Block-Level Striping, Q6: Help Command, Q8: Cross-Backend Copy, Q9: Compression, Q17: Test Context, Q18: Size() Limitation, Q19: Error Types, Q22: Parallel Reader Opening, Q23: StreamReconstructor Size Mismatch.


**Use this file to track decisions before they're made!** 🤔

