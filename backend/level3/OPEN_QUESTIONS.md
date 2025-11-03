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

## 🟡 Medium Priority

### Q2: Streaming Support for Large Files
**Question**: Should level3 support streaming to handle files larger than RAM?

**Context**:
- Current: Loads entire file into memory
- Works well for typical files (<1 GB)
- May fail for very large files (>available RAM)

**Options**:

**A) Keep current (memory buffering)**:
- ✅ Simple implementation
- ✅ Works for 99% of use cases
- ❌ Limited by RAM size

**B) Add streaming support**:
- ✅ Handle arbitrarily large files
- ✅ Better memory efficiency
- ❌ Complex implementation
- ❌ Hash calculation still needs full read

**C) Hybrid (stream + optional buffering)**:
- Use streaming for large files (>threshold)
- Use buffering for small files (fast)
- Most complex but most flexible

**Considerations**:
- Hash calculation requires full file read anyway
- Parity calculation needs all data
- Most cloud files are <1 GB

**Recommendation**: Keep current implementation until users report issues

**Who decides**: Based on user feedback

**Deadline**: None (wait for real-world usage)

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

## 🟢 Low Priority

### Q4: Configurable Write Policy
**Question**: Should users be able to choose degraded write mode?

**Context**: Some users might prefer high availability over consistency

**Decision**: Not implementing for now (keep simple)

**Reconsider if**: Users request this feature

**References**: `docs/ERROR_HANDLING_ANALYSIS.md` Option B

---

### Q5: Explicit Rebuild Command
**Question**: Should we add `rclone backend rebuild level3:` command?

**Context**: Self-healing handles automatic restoration, but explicit rebuild might be useful

**Use cases**:
- Manual intervention when self-healing fails
- Rebuild entire backend after restoring from backup
- Verification mode (rebuild and compare)

**Status**: Future enhancement (not needed now)

**References**: `docs/ERROR_HANDLING_POLICY.md`

---

### Q6: Move with Degraded Source
**Question**: Current behavior allows moving files with missing particles. Is this desired?

**Current**: Move succeeds, degraded state propagated to new location

**Options**:
- Keep current (flexible)
- Require all particles (strict)
- Reconstruct first (smart but slow)

**Decision**: Keep current, documented as known behavior

**Reconsider if**: Users report confusion or data loss

---

### Q7: Cross-Backend Move/Copy
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

## 📊 Statistics

**Total Open Questions**: 7  
**High Priority**: 1  
**Medium Priority**: 2  
**Low Priority**: 4  

**Decisions Made**: 8 (see `DESIGN_DECISIONS.md`)

---

**Use this file to track decisions before they're made!** 🤔

