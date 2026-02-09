# Open Questions - raid3 Backend

This document tracks open design questions and pending decisions for the raid3 backend, serving as a question registry (centralized list of issues requiring decisions), priority tracking (high/medium/low priority classification), status monitoring (active, resolved, or deferred questions), and decision workflow (process for moving questions to decisions). Process: Add questions as they arise, document decisions in [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md) when resolved. Last Updated: December 8, 2025. For resolved decisions, see [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md). For user documentation, see [`README.md`](../README.md).

---

## 🔴 High Priority

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

**Note**: In-process cache is not serialized; it only helps when the same process calls `About()` more than once (e.g. `rclone mount`, `rclone rcd`, or a long-running GUI). One-shot CLI invocations (`rclone about raid3:`) get no benefit because each run is a new process.

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

## 🟢 Low Priority

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

### Q26: Performance Test Option to Skip Largest File Size
**Status**: 🟢 **DEFERRED** - Not implementing for now  
**Priority**: Low

**Question**: Should `performance_test.sh` support an option to skip the test with the largest file size?

**Context**: The performance test uses multiple file sizes (4K, 40K, 400K, 4M, 40M, 4G). The largest size (4G) takes a long time to run (upload + download per iteration, multiple iterations). Users may want a quicker run by skipping the 4G test.

**Proposed Enhancement**: Add a CLI option (e.g. `--skip-largest` or `--max-size 40M`) so the script skips the largest file size when enabled.

**Benefit**: Shorter test runs when full 4G coverage is not needed.

**Implementation**: Deferred; add to open issues for future implementation.

**References**: 
- Script: `backend/raid3/test/performance_test.sh`
- File sizes: `FILE_SIZE_LABELS` (4K, 40K, 400K, 4M, 40M, 4G)

---

## 📋 Process for Resolving Questions

**Note**: Resolved questions are recorded in [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md).

### When a Question is Answered

Document the decision in [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md), update this file (remove question or move to resolved in DESIGN_DECISIONS), implement the decision in code, update user documentation if user-facing, and add tests if needed.

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

Total active questions: 15. Resolved questions moved to [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md) (Q2, Q4, Q5, Q7, Q20, Q24). Active by priority: High (2) - Q14: Health Check Caching, Q15: Background Worker Context. Medium (6) - Q1: Update Rollback, Q10: Backend Commands, Q11: DirMove Limitation, Q16: Configurable Values, Q21: Range Read Optimization, Q25: Usage/Quota Caching. Low (8) - Q6: Help Command, Q8: Cross-Backend Copy, Q9: Compression, Q18: Size() Limitation, Q19: Error Types, Q22: Parallel Reader Opening, Q23: StreamReconstructor Size Mismatch, Q26: Performance Test Skip Largest File.


**Use this file to track decisions before they're made!** 🤔

