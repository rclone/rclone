# Open Questions - Level3 Backend

**Purpose**: Track design questions that need decisions  
**Process**: Add questions as they arise, document decisions in `DESIGN_DECISIONS.md` when resolved  
**Last Updated**: December 7, 2025

---

## 🔴 High Priority

### Q1: Update Rollback Not Working Properly
**Status**: 🚨 **ACTIVE** - Still needs implementation
**Issue**: The `Update` operation does not properly rollback when rollback is enabled (`rollback=true`).

**Current Status**:
- ✅ Put rollback: Working correctly
- ✅ Move rollback: Working correctly  
- ❌ Update rollback: Not working properly

**Context**:
- Update operation uses a "move-to-temp" pattern when `rollback=true` (similar to chunker backend)
- When rollback is enabled, original particles are moved to temporary locations before applying updates
- If the update fails, particles should be restored from temp locations
- Current implementation may not be properly restoring particles on failure

**Impact**:
- Users with `rollback=true` (default) may experience incomplete updates if any particle update fails
- Can lead to degraded files (missing particles) which violates the all-or-nothing guarantee
- `README.md` currently documents that rollback works for "Put, Update, Move", but Update rollback needs to be fixed

**Next Steps**:
1. Investigate why Update rollback isn't working (debugging was started but reverted)
2. Test Update rollback scenarios similar to `move-fail` tests
3. Fix the rollback mechanism for Update operations
4. Update documentation to accurately reflect status

**Related Files**:
- `backend/level3/level3.go` - `Update()` and `updateWithRollback()` functions
- `backend/level3/tools/compare_level3_with_single_errors.sh` - May need `update-fail` tests

---

## 🟢 Low Priority

### Q6: Backend Help Command Behavior
**Status**: 🟢 **ACTIVE** - Low priority (help command is optional)
**Question**: How should `rclone backend help level3:` behave?

**Context**:
- Virtual backends can aggregate info (like `union`) OR show per-remote info (like `combine`)
- Level3 has 3 remotes with specific roles (even, odd, parity)
- Users might want to see capabilities or remote-specific details

**Options**:

**A) Aggregated (like union)**:
```
$ rclone backend help level3:
Features:
  - Combined capabilities of all 3 backends
  - Shows intersection of features
  - Overall level3 behavior
```

**B) Per-remote (like combine)**:
```
$ rclone backend help level3:
Even remote (minioeven):
  - Features of even backend
Odd remote (minioodd):
  - Features of odd backend
Parity remote (minioparity):
  - Features of parity backend
```

**C) Level3-specific (custom)**:
```
$ rclone backend help level3:
RAID 3 Backend
  - Byte-level striping with parity
  - Degraded mode: Reads work with 2/3 backends
  - Strict writes: All 3 backends required
  - See: rclone help level3
```

**Investigation**:
- [ ] Check how `union` backend implements help
- [ ] Check how `combine` backend implements help
- [ ] Determine what users would find most helpful

**Recommendation**: Start with Option C (level3-specific) - most informative

**Who decides**: You / maintainer

**Deadline**: None (low priority - help command is optional)

---

### Q2: Streaming Support for Large Files 🚨 **CRITICAL**
**Status**: 🚨 **ACTIVE** - Blocking production use with large files
**Question**: How should level3 handle large files (10+ GB) without loading entire file into memory?

**Updated**: 2025-11-03  
**Status**: 🚨 **CRITICAL ISSUE** - Blocking production use with large files

**Context**:
- **Current**: Uses `io.ReadAll()` - loads ENTIRE file into memory
- **Memory Usage**: ~3× file size (original + even + odd + parity + working)
- **Problem**: 10 GB file requires ~30 GB RAM (not feasible)
- **Major Backends**: S3, Google Drive, Mega all use streaming with chunks

**Memory Comparison**:

| File Size | Level3 (Current) | S3 Backend | Google Drive | Result |
|-----------|------------------|------------|--------------|--------|
| 100 MiB | ~300 MiB | ~5 MiB | ~8 MiB | ⚠️ Acceptable |
| 1 GB | ~3 GB | ~20 MiB | ~8 MiB | ⚠️ Marginal |
| 10 GB | ~30 GB | ~20 MiB | ~8 MiB | ❌ **FAILS** |
| 100 GB | ~300 GB | ~20 MiB | ~8 MiB | ❌ **IMPOSSIBLE** |

**Options**:

**A) Document limitation and keep current** (Short-term):
- ✅ No code changes needed
- ✅ Simple implementation stays
- ❌ Limited to ~500 MiB - 1 GB files
- ❌ Not suitable for video, backups, databases
- **Status**: ✅ DONE (README updated with warning)

**B) Implement chunk-level striping** (Long-term) ⭐ **RECOMMENDED**:
```go
chunkSize := 8 * 1024 * 1024  // 8 MiB
for {
    chunk := make([]byte, chunkSize)
    n, _ := io.ReadFull(in, chunk)
    
    evenChunk, oddChunk := SplitBytes(chunk[:n])
    parityChunk := CalculateParity(evenChunk, oddChunk)
    
    // Stream to particle writers
    evenWriter.Write(evenChunk)
    oddWriter.Write(oddChunk)
    parityWriter.Write(parityChunk)
}
```
- ✅ Constant memory (~32 MiB)
- ✅ Works with unlimited file sizes
- ✅ Maintains byte-level striping semantics
- ⚠️ Requires parallel writers (io.Pipe or OpenChunkWriter)

**C) Implement OpenChunkWriter** (Best) ⭐⭐ **BEST LONG-TERM**:
```go
func (f *Fs) OpenChunkWriter(...) (fs.ChunkWriterInfo, fs.ChunkWriter, error) {
    // Open chunk writers on all three backends
    evenWriter, _ := f.even.OpenChunkWriter(...)
    oddWriter, _ := f.odd.OpenChunkWriter(...)
    parityWriter, _ := f.parity.OpenChunkWriter(...)
    
    return &level3ChunkWriter{...}
}

func (w *level3ChunkWriter) WriteChunk(ctx, chunkNum int, reader io.ReadSeeker) (int64, error) {
    data, _ := io.ReadAll(reader)  // One chunk only (8 MiB)
    evenChunk, oddChunk := SplitBytes(data)
    parityChunk := CalculateParity(evenChunk, oddChunk)
    // Write chunks to all three backends
}
```
- ✅ Uses rclone's standard interface
- ✅ Supports resumable uploads
- ✅ Concurrent chunk uploads
- ✅ Compatible with S3/Drive patterns
- ⚠️ Requires backends to support OpenChunkWriter
- ⚠️ More complex implementation

**Investigation**:
- [x] Analyzed S3 backend → Uses multipart with 5 MiB chunks
- [x] Analyzed Google Drive → Uses resumable with 8 MiB chunks
- [x] Analyzed level3 current code → Uses io.ReadAll() ❌
- [x] Measured memory impact → 3× file size ❌
- [ ] Determine which backends support OpenChunkWriter
- [ ] Design level3ChunkWriter implementation
- [ ] Test with 10 GB files

**Recommendation**: 

**Immediate** (Do now):
- ✅ Document limitation in README (DONE)
- ✅ Add warning to users (DONE)
- Add test that verifies behavior with 100 MiB file
- Consider adding OPEN_QUESTIONS note

**Short-term** (Next sprint):
- Implement Option B (chunk-level striping with io.Pipe)
- Add `streaming_threshold` config option (default 100 MiB)
- Test with 1 GB files

**Long-term** (Future enhancement):
- Implement Option C (OpenChunkWriter)
- Add resumable upload support
- Test with 10+ GB files
- Remove file size limitation from README

**Priority**: 🚨 **HIGH** (critical for production use with large files)

**Who decides**: Maintainer / based on user requirements

**Deadline**: Before promoting to production use with large files

**References**: 
- See README.md for current file size limitations

---

## ✅ Resolved Questions

**Note**: These questions have been resolved and should be moved to `DESIGN_DECISIONS.md` for historical reference.

### Q4: Rebuild Command for Backend Replacement ✅ **IMPLEMENTED**
**Date**: 2025-11-02  
**Resolution Date**: 2025-12-07  
**Status**: ✅ **IMPLEMENTED** - The rebuild command is fully functional

**Original Question**: How should we implement RAID 3 rebuild when a backend is permanently replaced?

**Resolution**: 
The rebuild command has been fully implemented in `level3.go` (function `rebuildCommand` starting at line 1230). All proposed features are working:

✅ **Implemented Features**:
- Manual rebuild command: `rclone backend rebuild level3: [even|odd|parity]`
- Auto-detection: `rclone backend rebuild level3:` (auto-detects which backend needs rebuild)
- Check-only mode: `-o check-only=true`
- Dry-run mode: `-o dry-run=true`
- Priority options: `-o priority=auto|dirs-small|dirs|small`

**Documentation**: See `rclone backend help level3:` for full usage details. Also documented in `README.md` section "Backend Commands > Rebuild Command".

---

### Q5: Configurable Write Policy ✅ **RESOLVED - DECISION MADE**
**Status**: ✅ **RESOLVED** - Decision: Not implementing (keep simple)

**Original Question**: Should users be able to choose degraded write mode?

**Resolution**: Not implementing for now. Current strict write policy (all 3 backends required) matches hardware RAID 3 behavior and ensures data consistency. Keep implementation simple.

**Reconsider if**: Users request this feature

**References**: `docs/ERROR_HANDLING_ANALYSIS.md` Option B

---

### Q7: Move with Degraded Source ✅ **RESOLVED - DECISION MADE**
**Status**: ✅ **RESOLVED** - Decision: Keep current behavior (documented)

**Original Question**: Current behavior allows moving files with missing particles. Is this desired?

**Resolution**: Keep current behavior (flexible). Move succeeds even with degraded source, propagating degraded state to new location. This matches user expectations and avoids blocking moves unnecessarily.

**Documented**: This behavior is documented as known/expected.

**Reconsider if**: Users report confusion or data loss

---

## 🟢 Low Priority (Active Questions)

### Q3: Chunk/Block-Level Striping
**Status**: 🟢 **ACTIVE** - Low priority
**Question**: Should level3 support block-level striping instead of byte-level?

**Context**:
- Current: Byte-level (RAID 3 style)
- Alternative: Block-level (RAID 5 style)

**Why consider this**:
- Some backends might have minimum object size
- Fewer API calls (1 per block instead of 3 per file)
- Better for very small files

**Concerns**:
- More complex implementation
- Block size configuration needed
- Partial blocks at end of file

**Recommendation**: Stay with byte-level (simpler, true RAID 3)

**Who decides**: You / based on user needs

**Deadline**: None (current implementation works)

---


### Q8: Cross-Backend Move/Copy ⚠️ **NEEDS INVESTIGATION**
**Status**: 🟢 **ACTIVE** - Needs testing/investigation
**Question**: How should level3 handle copying FROM level3 TO level3?

**Context**: Same backend overlap issue as `union` and `combine`

**Current**: Likely fails with "overlapping remotes" error

**Investigation**: Test this scenario

**Who decides**: Based on testing results

---

### Q9: Compression Support with Streaming 🔮 **DECISION NEEDED**
**Status**: 🟢 **ACTIVE** - Research complete, awaiting decision
**Question**: Should level3 support optional compression (Snappy/Gzip) to reduce storage overhead, and how should it be implemented?

**Updated**: 2025-11-04  
**Status**: Research complete, awaiting decision on whether to implement  
**Related**: Q2 (Streaming - prerequisite)

**Decision Points**:
1. **Should we implement compression at all?** (Yes/No/Later)
2. **If yes, which algorithm?** (Snappy vs Gzip vs Both)
3. **How to implement?** (Architecture: Compress BEFORE or AFTER splitting)
4. **When to implement?** (After streaming support - Q2)

**Context**:
- Current: 150% storage overhead (even + odd + parity)
- With Snappy: 75% overhead for text (50% savings!)
- With Gzip: 50% overhead for text (67% savings!)
- **Critical**: Requires streaming support first (Q2)

**⚠️ CRITICAL INSIGHT** (from user):
**Compression order affects entropy and compression ratio!**
- ✅ **Correct**: Compress BEFORE splitting (preserves patterns, 2× ratio)
- ❌ **Wrong**: Compress AFTER splitting (increases entropy, 1.5× ratio)
- **Impact**: Correct order gives **2× better savings** (50% vs 23%)!

**Why Entropy Matters**:
```
Original: "The quick brown fox..."
  → Patterns: "The quick", "brown", repeating words
  → Compression: 2× ratio ✅

After byte-striping:
  Even: "T u c  r w  o ..." (fragmented)
  Odd: "hqikbon..." (high entropy)
  → Patterns broken!
  → Compression: 1.5× ratio ⚠️ (40% worse)
```

**Correct Architecture**:
```
Compress(original) → Split(compressed bytes) → Parity(compressed) → Store
Reconstruction: Merge(compressed bytes) → Decompress → Original
```

**Options**:

A) **Snappy** (Recommended) ⭐⭐⭐:
- ✅ Speed: 250-500 MB/s (matches RAID 3 philosophy)
- ✅ CPU: Very low (5-10%)
- ✅ Framing: Native RFC 8478 (perfect for streaming)
- ✅ Savings: 50% for text, 10% for binary
- ⚠️ Ratio: Moderate (1.5-2×)

B) **Gzip** (Better ratio, slower):
- ✅ Ratio: 2.5-3.5× (better than Snappy)
- ✅ Savings: 67% for text
- ⚠️ Speed: 50-100 MB/s (slower)
- ⚠️ CPU: Moderate-high (30-80%)
- ⚠️ Framing: Needs sgzip for random access

C) **Both** (Configurable):
- User chooses: `compress=true, type=snappy|gzip`
- Best flexibility
- More complex implementation

D) **None** (Current):
- Simple implementation
- No compression overhead
- Higher storage costs

**Recommendation** (if implementing): 
- **Algorithm**: Snappy ⭐⭐⭐ (speed matches RAID 3 philosophy)
  - Alternative: Gzip (better ratio but slower)
  - Future: Make configurable (both options)
- **Architecture**: ✅ **MUST compress BEFORE splitting** (critical!)
  - Compress(original) → Split(compressed) → Parity → Store
  - **NOT**: Split → Compress particles (destroys patterns, 40% worse!)
- **Implementation Order**:
  1. Phase 1: Streaming support (Q2) - **PREREQUISITE**
  2. Phase 2: Add Snappy compression - optional feature
  3. Phase 3: Add Gzip alternative - optional
  4. Phase 4: Make algorithm configurable

**Investigation**: ✅ **COMPLETE**
- [x] Research Snappy vs Gzip algorithms
- [x] Compare with rclone compress backend
- [x] Analyze entropy impact of byte-striping ⭐ **CRITICAL INSIGHT**
- [x] Document correct compression order (compress BEFORE split!)
- [x] Calculate storage savings (50% for text vs 23% if wrong order)
- [x] Design streaming architecture
- **Documentation**: 
  - `docs/COMPRESSION_ANALYSIS.md` (full comparison)

**Key Implementation Decisions to Make**:
1. **Implement compression?** 
   - ✅ Pros: 50% storage savings, lower bandwidth
   - ❌ Cons: Added complexity, CPU overhead
   - **Decision**: TBD

2. **Which algorithm first?**
   - Option A: Snappy only (simpler, fast)
   - Option B: Gzip only (better ratio, slower)
   - Option C: Both, configurable (most flexible)
   - **Decision**: TBD (recommend Snappy first)

3. **Configuration approach?**
   - Option A: Backend parameter: `compress=true`
   - Option B: Transparent (always compress)
   - Option C: Per-file metadata flag
   - **Decision**: TBD (recommend Option A)

**Priority**: Low (requires Q2 streaming first, but high value if implemented)

**Who decides**: Project maintainer + user feedback on storage costs

**Deadline**: None (future enhancement after streaming)

**Estimated Effort**: 
- Streaming (Q2): 20-30 hours (prerequisite)
- + Snappy implementation: 10-15 hours
- + Configuration options: 5 hours
- + Gzip alternative (optional): 8 hours
- **Total**: ~35-58 hours (depending on scope)

---

### Q10: Backend-Specific Commands Support 🤝 **COMMUNITY DISCUSSION**
**Status**: 🟡 **ACTIVE** - Awaiting rclone community discussion
**Question**: Should level3 support backend-specific commands when all three remotes use the same backend type?

**Updated**: 2025-11-04  
**Status**: Research complete, awaiting rclone community discussion  
**Related**: rclone backend command framework

**Context**:
- rclone backends can implement custom commands via `fs.Commander` interface
- Examples: S3 `restore`, Drive `shortcut`, B2 `lifecycle`, etc.
- level3 has 3 remotes - should commands affect all three?
- **Critical use case identified**: S3 object tagging for lifecycle/billing/access control

**Key Finding**:
**S3 Object Tags are CRITICAL for level3** 🏷️
- Lifecycle policies use tags (archive old files)
- Cost allocation uses tags (billing by project)
- IAM policies use tags (access control)
- **Problem**: Tags must be consistent across all three particles!
- **Without tag commands**: Manual tagging → risk of inconsistency → broken lifecycle policies

**Real-World Scenario**:
```
Company tags files for archival: "Lifecycle=Archive"
S3 lifecycle rule: Move tagged objects to GLACIER after 30 days

Without level3 tag support:
  - User manually tags even particle ✅
  - User forgets to tag odd particle ❌
  - User forgets to tag parity particle ❌
  
Result:
  - Only even particle archived to GLACIER
  - odd and parity stay in standard storage
  - Reconstruction FAILS (even is archived, odd is not)
  - level3 is BROKEN! ⚠️⚠️⚠️
```

**Options**:

A) **No Support**:
- Simple (no implementation)
- ❌ Missing critical functionality (tag management!)
- ❌ Risk of broken lifecycle policies
- ❌ Can't rotate credentials easily

B) **Support Subset** (Recommended) ⭐⭐⭐:
- Commands to support:
  1. **`set`** - Update config on all 3 remotes (credential rotation)
  2. **`get`** - Get config from backends
  3. **`settags`** 🏷️ - Set object tags (lifecycle/billing/access) **CRITICAL**
  4. **`gettags`** 🏷️ - Get object tags
  5. **`deletetags`** 🏷️ - Delete object tags
  6. **`cleanup`** - Clean up orphaned particles
  7. **`versioning`** - Version management (with concatenated version IDs)
- ✅ Solves real problems (tags, credentials, cleanup)
- ✅ Maintains level3 abstraction
- ⚠️ Moderate implementation effort (~22h Phase 1, ~33h Phase 2)

C) **Support All Commands**:
- ❌ Breaks abstraction
- ❌ Unclear semantics for many commands
- ❌ High maintenance burden

**Investigation**: ✅ **COMPLETE**
- [x] Inventory all backend commands (15 backends analyzed)
- [x] Identify common patterns (config, cleanup, tags, versioning)
- [x] Analyze tag command criticality
- [x] Research S3 tag support in rclone
- [x] Design implementation approach
- [x] Evaluate versioning with concatenated IDs
- **Documentation**: `docs/BACKEND_COMMANDS_ANALYSIS.md` (30KB)

**Key Insights**:
1. **Tags are CRITICAL** - Not optional, required for production use
2. **Version IDs can be concatenated** - `even_id|odd_id|parity_id` approach works
3. **Config commands useful** - Credential rotation across all 3 remotes
4. **Backend-specific features** - No abstraction possible (Drive shortcuts, etc.)

**Commands Analysis**:

| Command | Backends | level3 Priority | Why? |
|---------|----------|-----------------|------|
| `settags` 🏷️ | S3, Azure | **HIGH ⭐⭐⭐** | Lifecycle/billing/access CRITICAL |
| `gettags` 🏷️ | S3, Azure | **HIGH ⭐⭐⭐** | Tag inspection/validation |
| `set` | S3, Drive, HTTP | **HIGH ⭐⭐⭐** | Credential rotation |
| `deletetags` 🏷️ | S3, Azure | **MEDIUM ⭐⭐** | Tag management |
| `get` | S3, Drive, HTTP | **MEDIUM ⭐⭐** | Config inspection |
| `cleanup` | S3, B2 | **MEDIUM ⭐⭐** | Housekeeping |
| `versioning` | S3, B2 | **MEDIUM ⭐⭐** | Version control (concatenated IDs) |
| `shortcut`, `copyid` | Drive | **NO** ❌ | Backend-specific, no abstraction |

**Recommendation**: 
- **Phase 1**: Implement `set`, `settags`, `gettags`, `deletetags`, `get`, `cleanup` (~22h)
- **Phase 2**: Implement `versioning` with concatenated version IDs (~30h)
- **Discuss with rclone community**:
  * Is this approach sound?
  * Should tag commands be in rclone core instead?
  * Any security/architectural concerns?
  * API design feedback

**Priority**: Medium-High (tags are critical for production, but need community input first)

**Who decides**: rclone community discussion + maintainers

**Deadline**: None (community discussion first)

**Estimated Effort**: 
- Phase 1 (essential commands): ~22 hours
- Phase 2 (versioning): ~30 hours
- Total: ~52 hours

**Next Steps**:
1. Create GitHub discussion/issue in rclone repository
2. Present backend commands analysis
3. Emphasize S3 tagging criticality
4. Get community feedback on approach
5. Refine design based on feedback
6. Implement if approved

---

## 📋 Process for Resolving Questions

### When a Question is Answered:

1. **Document the decision** in `DESIGN_DECISIONS.md`
2. **Update this file** - move question to "Resolved" section or delete
3. **Implement the decision** in code
4. **Update user documentation** if user-facing
5. **Add tests** if needed

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

**Who decides**: [Person/process]

**Deadline**: [Date or "None"]
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

**Deadline**: [Date or "None"]
```

---

## 🟡 Medium Priority

### Q11: Bucket/Directory Renaming (DirMove) Limitation with S3
**Status**: 🟡 **ACTIVE** - Known limitation, may improve error messages
**Question**: How should level3 handle directory/bucket renaming when underlying backends don't support DirMove?

**Date Added**: November 6, 2025  
**Status**: Limitation identified during testing  
**Related**: S3/Minio backend limitations

**Context**:
- User attempted `rclone moveto miniolevel3:mybucket miniolevel3:mybucket2` to rename a bucket
- S3/Minio backends don't support bucket renaming (fundamental S3 API limitation)
- S3 backend doesn't implement `DirMove` at all - it's not just level3!
- level3 now implements `DirMove` but it only works if **all three** underlying backends support it

**Current Behavior**:
```bash
$ rclone moveto miniolevel3:mybucket miniolevel3:mybucket2
# Falls back to file-by-file copy + delete
# For empty buckets: creates destination bucket but source remains
```

**Why This Happens**:
1. S3/Minio backends don't implement `DirMove` (S3 API limitation)
2. level3's `DirMove` requires all 3 backends to support it
3. rclone falls back to copy + delete (slow for large directories)
4. Buckets can't be renamed in S3 (need to create new + copy + delete old)

**What WOULD Work**:
- Directory renaming **within** buckets (if backend supported DirMove)
- Bucket renaming with backends like:
  - Local filesystem
  - SFTP
  - FTP
  - Any backend that implements DirMove

**Options**:

**A) Document as Known Limitation** (Current) ⭐ **RECOMMENDED**:
- ✅ No code changes needed
- ✅ Matches S3 behavior (can't rename buckets)
- ✅ Clear in documentation
- ⚠️ Users need to know: copy + delete instead of rename
- **Status**: Already implemented

**B) Improve Error Message**:
```bash
$ rclone moveto miniolevel3:mybucket miniolevel3:mybucket2
ERROR: Cannot rename buckets with S3 backends (S3 API limitation)
TIP: Use 'rclone copy' + 'rclone purge' instead for large datasets
```
- ✅ Clear guidance to users
- ✅ Explains it's an S3 limitation, not level3
- ⚠️ Need to detect bucket-level vs directory-level operations

**C) Implement Special Handling for Empty Buckets**:
- Create destination bucket on all 3 backends
- Delete source bucket on all 3 backends  
- Only works for empty buckets
- ⚠️ Complex, limited benefit

**D) No Action** (Accept Limitation):
- S3 fundamentally doesn't support bucket renaming
- Users of S3 are already familiar with this limitation
- level3 behaves same as underlying S3 backend

**Investigation**:
- [x] Confirmed S3 backend has no DirMove implementation
- [x] Tested bucket renaming behavior
- [x] Verified level3 DirMove implementation works (when backends support it)
- [ ] Test directory renaming within buckets (with supporting backends)
- [ ] Add documentation to README about this limitation
- [ ] Consider improved error messages

**Real-World Impact**:
- **Bucket renaming**: Not supported (fundamental S3 limitation)
- **Directory renaming**: Would work with DirMove-capable backends
- **Workaround**: `rclone copy source dest` + `rclone purge source`

**Recommendation**: 
- **Short-term**: Document as known limitation (Option A)
- **Medium-term**: Add to README/FAQ with workaround instructions
- **Long-term**: Consider improved error message (Option B) if users frequently encounter this

**Priority**: 🟡 **Low-Medium** (S3 limitation users are familiar with)

**Who decides**: Maintainer (document vs improve error)

**Deadline**: None (known limitation, not critical)

**Related Issues**:
- S3 doesn't support bucket renaming at all
- This affects all S3-based multi-backend systems
- Workaround exists (copy + delete)

---


## 📊 Statistics

**Total Active Questions**: 8  
**Resolved Questions**: 3 (Q4, Q5, Q7 - see "Resolved Questions" section above)

**Active Questions by Priority**:
- 🔴 **High Priority**: 1 
  - Q2: Streaming Support for Large Files 🚨 **CRITICAL**
- 🟡 **Medium Priority**: 2
  - Q1: Update Rollback Not Working Properly
  - Q10: Backend-Specific Commands Support 🤝 (awaiting community discussion)
  - Q11: Bucket/Directory Renaming (DirMove) Limitation
- 🟢 **Low Priority**: 5
  - Q3: Chunk/Block-Level Striping
  - Q6: Backend Help Command Behavior
  - Q8: Cross-Backend Move/Copy
  - Q9: Compression Support with Streaming 🔮

**Question Numbering**:
- Active: Q1, Q2, Q3, Q6, Q8, Q9, Q10, Q11
- Resolved: Q4 (Rebuild Command - IMPLEMENTED), Q5 (Configurable Write Policy), Q7 (Move with Degraded Source)  

**Decisions Made**: See `DESIGN_DECISIONS.md` for resolved questions

**Critical Issues**: 1 (Q2: Large file streaming - blocking production use with >1 GB files)

**Research Complete - Awaiting Decisions**: 
- Q9 (Compression with Snappy) - Research complete, decision needed on:
  * Whether to implement at all
  * Which algorithm (Snappy vs Gzip vs both)
  * Configuration approach
  * Prerequisite: Q2 streaming support
- Q10 (Backend Commands & Tags) - Research complete, awaiting rclone community discussion:
  * Whether to support backend commands
  * Which commands to implement (tags are critical!)
  * API design approach

---

**Use this file to track decisions before they're made!** 🤔

