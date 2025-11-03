# Open Questions - Level3 Backend

**Purpose**: Track design questions that need decisions  
**Process**: Add questions as they arise, document decisions in `DESIGN_DECISIONS.md` when resolved  
**Last Updated**: November 2, 2025

---

## 🔴 High Priority

### Q1: Backend Help Command Behavior
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

## 🔴 High Priority

### Q2: Streaming Support for Large Files 🚨 **CRITICAL**
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
- `docs/LARGE_FILE_ANALYSIS.md` (comprehensive analysis) ⭐ **NEW**

---

---

### Q3: Chunk/Block-Level Striping
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

### Q4: Rebuild Command for Backend Replacement
**Date**: 2025-11-02  
**Question**: How should we implement RAID 3 rebuild when a backend is permanently replaced?

**Context**:
- Hardware RAID 3 has a rebuild process: failed drive → replace → rebuild → healthy
- Level3 needs similar process: failed backend → replace → ??? → healthy
- Currently: Self-healing is automatic/opportunistic, not complete/deliberate

**Scenario**:
```
Day 0: Odd backend fails (disk crash, account closed, etc.)
  → Self-healing handles reads ✅
  → Writes blocked (strict policy) ❌
  
Day 1: User creates new odd backend (empty)
  → Updates rclone.conf
  → Needs to restore ALL particles to new backend
  → How to do this?
```

**Options**:

**A) Manual Rebuild Command** (Recommended):
```bash
# Check what needs rebuild
rclone backend rebuild level3: -o check-only=true

# Rebuild specific backend
rclone backend rebuild level3: odd

# With options
rclone backend rebuild level3: odd -o priority=small
```

**B) Automatic Detection**:
```bash
# Auto-detect which backend needs rebuild
rclone backend rebuild level3:
# Prompts: "Odd backend missing 1,247 files. Rebuild? (y/n)"
```

**C) Use rclone sync** (doesn't work):
```bash
rclone sync level3: new-odd:
# ❌ Would sync merged files, not particles
# ❌ Doesn't understand particle structure
```

**Proposed Features**:

**MVP (Phase 1)**:
- Basic rebuild command
- Progress display
- Verification

**Advanced (Phase 2)**:
- Check/analysis mode (`-o check-only=true`)
- Priority options (`-o priority=dirs|small|large`)
- Size filtering (`-o max-size=100M`)
- Dry-run (`-o dry-run=true`)

**Verification (Phase 3)**:
- Separate verify command
- Health status report

**Terminology Decision**:
- ✅ **"Rebuild"** (standard RAID term, not "recover")
- Used by: mdadm, hardware RAID, ZFS (resilver), industry standard

**Implementation Complexity**:
- MVP: ~200 lines, 4-6 hours
- Full: ~1,250 lines, 14-23 hours
- Reuses existing: SplitBytes(), MergeBytes(), CalculateParity()

**vs. Self-Healing**:
- Self-healing: Automatic, opportunistic, gradual (during reads)
- Rebuild: Manual, complete, fast (dedicated process)
- **Both needed!** Different use cases

**Investigation**:
- [x] Research RAID terminology → "Rebuild" is standard
- [x] Check rclone backend command pattern → `fs.Commander` interface
- [x] Design command structure → Single command with options
- [ ] Decide if/when to implement

**User-Centric Approach** (NEW):

The concern: Users may not know about `backend` commands or how to discover them!

**Better UX - Multi-Layer Discovery**:

1. **Enhanced Error Messages** (when write fails):
   ```
   ERROR: Cannot write - level3 DEGRADED (odd unavailable)
   Diagnose: rclone backend status level3:
   ```

2. **`status` Command** (central diagnostic tool):
   ```bash
   $ rclone backend status level3:
   # Shows: Full health report + step-by-step recovery guide
   ```

3. **`rebuild` Command** (for actual rebuild):
   ```bash
   $ rclone backend rebuild level3: odd
   # Or: rclone backend status level3: --rebuild
   ```

**Benefits**:
- ✅ Error messages guide users
- ✅ `status` command provides complete recovery guide
- ✅ No RAID knowledge required
- ✅ Natural discovery path

**Recommendation**: 
- **Phase 1**: Enhanced error messages (1 hour) ⭐ **Do first**
- **Phase 2**: `status` command (3-4 hours) ⭐ **Critical for UX**
- **Phase 3**: `rebuild` command (4-6 hours) ⭐ **Completes workflow**

**Total effort**: 8-11 hours for excellent UX

**Priority**: 🟡 **Medium-High** (critical for production usability)

**Who decides**: You (maintainer)

**References**: 
- `docs/REBUILD_RECOVERY_RESEARCH.md` (technical analysis)
- `docs/USER_CENTRIC_RECOVERY.md` (UX analysis) ⭐ **NEW**

---

## 🟢 Low Priority

### Q5: Configurable Write Policy
**Question**: Should users be able to choose degraded write mode?

**Context**: Some users might prefer high availability over consistency

**Decision**: Not implementing for now (keep simple)

**Reconsider if**: Users request this feature

**References**: `docs/ERROR_HANDLING_ANALYSIS.md` Option B

---

### Q6: Explicit Rebuild Command
**Status**: ⚠️ **SUPERSEDED by Q4**

**Note**: This question is now fully addressed in Q4 with comprehensive research.

See: Q4 (Rebuild Command for Backend Replacement) and `docs/REBUILD_RECOVERY_RESEARCH.md`

---

### Q7: Move with Degraded Source
**Question**: Current behavior allows moving files with missing particles. Is this desired?

**Current**: Move succeeds, degraded state propagated to new location

**Options**:
- Keep current (flexible)
- Require all particles (strict)
- Reconstruct first (smart but slow)

**Decision**: Keep current, documented as known behavior

**Reconsider if**: Users report confusion or data loss

---

### Q8: Cross-Backend Move/Copy
**Question**: How should level3 handle copying FROM level3 TO level3?

**Context**: Same backend overlap issue as `union` and `combine`

**Current**: Likely fails with "overlapping remotes" error

**Investigation**: Test this scenario

**Who decides**: Based on testing results

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

---

## 📊 Statistics

**Total Open Questions**: 7  
**High Priority**: 2 (Q1: Backend Help, Q2: Streaming 🚨)  
**Medium Priority**: 1 (Q4: Rebuild Command)  
**Low Priority**: 4  

**Decisions Made**: 8 (see `DESIGN_DECISIONS.md`)

**Critical Issues**: 1 (Q2: Large file streaming - blocking production use with >1 GB files)

---

**Use this file to track decisions before they're made!** 🤔

