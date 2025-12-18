# Open Questions - raid3 Backend

This document tracks open design questions and pending decisions for the raid3 backend, serving as a question registry (centralized list of issues requiring decisions), priority tracking (high/medium/low priority classification), status monitoring (active, resolved, or deferred questions), and decision workflow (process for moving questions to decisions). Process: Add questions as they arise, document decisions in [`DESIGN_DECISIONS.md`](DESIGN_DECISIONS.md) when resolved. Last Updated: December 8, 2025. For resolved decisions, see [`DESIGN_DECISIONS.md`](DESIGN_DECISIONS.md). For user documentation, see [`README.md`](../README.md).

---

## 🔴 High Priority

### Q2: Streaming Support for Large Files 🚨 **CRITICAL**
**Status**: 🚨 **ACTIVE** - Blocking production use with large files  
**Priority**: High

Current implementation loads entire files into memory (~3× file size). 10 GB file requires ~30 GB RAM (not feasible). Implement streaming using rclone's buffer pool (`lib/pool`), chunk-level striping with io.Pipe, or OpenChunkWriter. Prerequisite for compression support (Q9).

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

**Context**: Current storage overhead is 150% (even + odd + parity). Compression could reduce this significantly (e.g., ~75% overhead with Snappy for text files, ~50% savings). **Critical**: Must compress BEFORE splitting to preserve patterns and achieve good compression ratio (compressing after splitting destroys patterns and reduces ratio by ~40%). Requires streaming support first (Q2 - prerequisite). Options: Snappy (fast, low CPU), LZ4 (very fast, low CPU), or configurable. Decision needed: whether to implement, which algorithm, and configuration approach.

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

## ✅ Resolved Questions

**Note**: These questions have been resolved and should be moved to [`DESIGN_DECISIONS.md`](DESIGN_DECISIONS.md) for historical reference.

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

Document the decision in [`DESIGN_DECISIONS.md`](DESIGN_DECISIONS.md), update this file (move question to "Resolved" section or delete), implement the decision in code, update user documentation if user-facing, and add tests if needed.

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

Total active questions: 14. Resolved questions: 4 (Q4, Q5, Q7, Q20). Active questions by priority: High Priority (3) - Q2: Streaming Support, Q14: Health Check Caching, Q15: Background Worker Context. Medium Priority (5) - Q1: Update Rollback, Q10: Backend Commands, Q11: DirMove Limitation, Q12: Post-Rename Verification, Q16: Configurable Values. Low Priority (6) - Q3: Block-Level Striping, Q6: Help Command, Q8: Cross-Backend Copy, Q9: Compression, Q17: Test Context, Q18: Size() Limitation, Q19: Error Types. Critical issues: 1 (Q2: Large file streaming blocking production use).


**Use this file to track decisions before they're made!** 🤔

