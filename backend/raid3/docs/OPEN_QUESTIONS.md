# Open Questions - raid3 Backend

This document tracks open design questions and pending decisions for the raid3 backend, serving as a question registry (centralized list of issues requiring decisions), priority tracking (high/medium/low priority classification), status monitoring (active, resolved, or deferred questions), and decision workflow (process for moving questions to decisions). Process: Add questions as they arise, document decisions in [`../_analysis/DESIGN_DECISIONS.md`](../_analysis/DESIGN_DECISIONS.md) when resolved. **Last reviewed**: 2026-02-12. For user documentation, see [`README.md`](../README.md).

**Review 2026-02-12**: All 17 open items were checked against the codebase. **Q15** (background worker context) is **resolved**: workers use `uploadCtx` derived from `NewFs` and respect `ctx.Done()`. **Q18** is partially done (Size() limitation documented in code). **Q21** and **Q22** line references were updated to current code. **Q25** line reference simplified. No other questions were removed; design/limitation items (Q6, Q8, Q9, Q10, Q11, etc.) remain relevant.

---

## 🔴 High Priority

---

### Q14: Optimize Health Checks (Add Caching)
**Status**: 🔴 **ACTIVE** - Performance optimization  
**Priority**: High (affects write performance)

Health checks run before every write operation, causing network I/O overhead. Add TTL-based caching to reduce redundant checks and improve write performance.

---

### Q15: Make Background Worker Context Respect Cancellation
**Status**: ✅ **RESOLVED** (2026-02-12)  
**Priority**: High (affects graceful shutdown)

Background upload workers now use a context derived from the parent passed to `NewFs`: `f.uploadCtx, f.uploadCancel = context.WithCancel(ctx)` in `raid3.go`, and `backgroundUploader(ctx, …)` in `heal.go` exits on `<-ctx.Done()`. Workers therefore respect cancellation for graceful shutdown. Move to DESIGN_DECISIONS when that doc exists.

---

## 🟡 Medium Priority

### Q27: Implement Heal-on-Read for Streaming Path (Tests: TestHeal, TestHealEvenParticle, TestHealLargeFile)
**Status**: 🟡 **ACTIVE** - Implement to re-enable skipped tests  
**Priority**: Medium

After dropping the buffered path, the backend is streaming-only. The **heal-on-read** behaviour (queue missing particle for background upload when opening in degraded mode) was only implemented in the removed buffered open path. The streaming path does not call `queueParticleUpload` when reconstructing via `StreamReconstructor`, so auto-heal on read no longer runs.

**Tests currently skipped** (add to implement list so they can be re-enabled):
- `TestHeal` – odd particle restored by heal after Open+Read and Shutdown
- `TestHealEvenParticle` – even particle restored by heal after Open+Read and Shutdown
- `TestHealLargeFile` – same for a larger file (100 KB)

**Implementation options**: (1) When returning a `StreamReconstructor` from `openStreaming`, wrap it in a reader that buffers the reconstructed stream and on `Close()` (or when EOF is seen) reconstructs the missing particle and calls `queueParticleUpload`. (2) Or run a separate goroutine that tees the reconstructed output to a buffer and queues the upload when done. The challenge is that the streaming path does not have the full reconstructed bytes in memory until the stream is consumed.

**References**: `backend/raid3/object.go` (`openStreaming` degraded branch), `backend/raid3/raid3_heal_test.go` (skipped tests).

---

### Q28: Fix or Document TestReadSucceedsWithUnavailableBackend (Odd+Parity Reconstruction)
**Status**: 🟡 **ACTIVE** - Investigation / fix  
**Priority**: Medium

**Observed**: With the streaming-only path, `TestReadSucceedsWithUnavailableBackend` fails on the **odd+parity** reconstruction case (even particle missing). The test expects 35 bytes (`"Should be readable in degraded mode"`) but gets 34 bytes; the last byte is wrong (e.g. `0x0b 0x64` instead of `"ode"`). So the last logical byte is lost or corrupted when reconstructing from odd (17 bytes) + parity (18 bytes) for an odd-length file.

**Why it happens**: For a 35-byte logical file, even=18, odd=17, parity=18. In `StreamReconstructor` we read from the odd particle (data) and parity particle in parallel. We then call `ReconstructFromOddAndParity(oddData, parityData, isOddLength)`, which expects `len(parity) >= len(odd)` and for odd-length uses the last parity byte as the last even byte. If the first `Read()` from parity returns **17 bytes and EOF** (instead of 18), we pass 17 and 17 to `ReconstructFromOddAndParity`, which then produces 34 bytes and the last byte is wrong. So the root cause is that the **parity reader is returning 17 bytes and EOF** for an 18-byte file on the first read.

**Places to verify**:
1. **Actual read lengths**: Add temporary debug in `StreamReconstructor.Read()` to log `dataN` and `parityN` (and `dataRes.hitEOF`, `parityRes.hitEOF`) on the first iteration when both are small. Confirm whether parity returns 17 or 18.
2. **Underlying reader**: The parity particle is opened with `parityObj.Open(ctx, filteredOptions...)` (no range). For the local backend this is typically an `*os.File`. A single `Read(buf)` with `len(buf) > 18` should return 18 and `nil` (or 18 and EOF). If it returns 17 and EOF, the cause may be backend-specific or a wrapper.
3. **Chunk size**: Default `chunkSize` is 8 MiB; for a 18-byte file the first read should get all 18 bytes. If any test or config forces a smaller chunk size, that could change behaviour.

**Next steps**: Run with debug logging to confirm `dataN`/`parityN`; if parity really returns 17+EOF, either fix the reader behaviour or add a workaround in `StreamReconstructor` (e.g. ignore EOF when we expect one more byte for odd-length odd+parity and try one more read).

**References**: `backend/raid3/particles.go` (`StreamReconstructor`), `backend/raid3/raid3_errors_test.go` (`TestReadSucceedsWithUnavailableBackend` – currently skipped).

---

### Q1: Update Rollback Not Working Properly
**Status**: ✅ **RESOLVED** (2026-02-21)  
**Priority**: Medium

**Resolution**: Update now uses `rollbackPut` on failure: removes successfully updated particles instead of the temp-based `rollbackUpdate` (which did not apply to in-place Update). The object is left degraded but consistent (remaining particles have old content); rebuild/heal can restore. See `object.go` updateStreaming defer.

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
**Status**: ✅ **RESOLVED** - Implemented  
**Priority**: Medium

**Question**: Should range reads apply byte ranges directly to particle readers instead of reading entire particles and filtering?

**Resolution**: Implemented. Block-level range reads fetch only the required compressed blocks (or uncompressed blocks) from particles instead of streaming full particles. When a range is requested, the implementation:
- Computes block indices from the requested byte range
- Derives particle byte ranges via `fullStreamRangeForBlocks` + `particleRangesForFullStream`
- Opens particles with `RangeOption{Start: particleStart, End: particleEnd}`
- Uses `rangeFilterReader` only for sub-block trimming

Works for both compressed (block-based) and uncompressed data (same 128 KiB block structure via `uncompressedInventory`), in normal and degraded mode. See `docs/RANGE_READ_IMPLEMENTATION_CHECKLIST.md`.

---

### Q25: Usage/Quota Caching with Aggregation
**Status**: 🟡 **ACTIVE** - Performance optimization  
**Priority**: Medium

**Question**: Should raid3 implement cached quota information with TTL-based expiration similar to union backend?

**Context**: Currently, `About()` method queries all three backends every time it's called, causing network I/O overhead and latency. The union backend implements sophisticated quota caching with background updates that could be adapted for raid3's aggregated usage reporting.

**Current Implementation** (`raid3.go`, `About` method):
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

### Q29: About Aggregation Semantics — Is Summing Proper?
**Status**: 🟡 **ACTIVE** - Design discussion  
**Priority**: Medium

**Question**: Is the current About aggregation behavior correct?

**Current behavior**: About aggregates quota/usage (Total, Used, Trashed, Other, Free, Objects) by **summing** these values from the even, odd, and parity backends.

**Context**: raid3 stores 3× the data (one full copy on even, one on odd, one parity block). Summing Total/Used/Free across all three backends yields the combined physical storage across the remotes. Whether this is the right semantic for users (e.g. `rclone about raid3:`) needs discussion — alternatives might include reporting logical usage, per-backend breakdown, or different aggregation rules.

**References**: `backend/raid3/raid3.go` (About method, ~lines 800-857), `_analysis/RAID3_COMMAND_COVERAGE_REVIEW.md` (Q&A 6.2).

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

### Q9: Compression Support with Streaming
**Status**: ✅ **RESOLVED** - Implemented (Snappy and Zstd)  
**Question**: Should raid3 support optional compression (Snappy/LZ4) to reduce storage overhead?

**Resolution**: Implemented. The backend supports optional compression via the `compression` option: `none` (default), `snappy`, or `zstd`. Data is compressed after hashing and before splitting; reads use the footer's Compression field to decompress. Snappy is fast with moderate ratio; zstd gives better compression at a default level. See README Configuration.

---

### Q18: Document Size() Context Limitation
**Status**: 🟢 **ACTIVE** - Documentation improvement  
**Priority**: Low

`Size()` method doesn't accept context parameter (matches rclone interface); internal operations use `context.Background()` which can't be cancelled. **Already documented in code**: `object.go` has a comment above `Size()` (interface limitation, use of Background). Optional: add a short note in README "Limitations" if desired.

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

**Context**: In `openStreaming()` (`object.go`), object lookup is already parallel, but reader opening for the healthy (both particles present) path is sequential (e.g. `evenObj.Open` then `oddObj.Open` at ~476-485). This adds small latency overhead.

**Proposed Optimization**:
- Open both readers concurrently using `errgroup`
- Small latency improvement (typically <10ms per read operation)

**Benefit**: Minor latency improvement for read operations

**Implementation Complexity**: Low (1-2 hours)
- Simple change to use errgroup for parallel opening
- Already have pattern from other concurrent operations

**References**: 
- Current implementation: `backend/raid3/object.go` (openStreaming, even/odd Open calls)
- Similar pattern: `backend/raid3/raid3.go` (concurrent uploads)

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

Total active questions: 16. Resolved in this file or moved to DESIGN_DECISIONS: Q2, Q4, Q5, Q7, Q15 (resolved 2026-02-12: background workers use context from NewFs), Q20, Q24. Active by priority: High (1) - Q14: Health Check Caching. Medium (8) - Q1: Update Rollback, Q10: Backend Commands, Q11: DirMove Limitation, Q16: Configurable Values, Q21: Range Read Optimization, Q25: Usage/Quota Caching, Q27: Heal-on-Read for Streaming (implement TestHeal*), Q28: TestReadSucceedsWithUnavailableBackend (odd+parity). Low (8) - Q6: Help Command, Q8: Cross-Backend Copy, Q9: Compression, Q18: Size() Limitation (code comment done; README optional), Q19: Error Types, Q22: Parallel Reader Opening, Q23: StreamReconstructor Size Mismatch, Q26: Performance Test Skip Largest File.


**Use this file to track decisions before they're made!** 🤔

