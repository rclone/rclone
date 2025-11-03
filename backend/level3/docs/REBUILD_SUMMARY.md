# Rebuild Command - Research Summary

**Date**: November 2, 2025  
**Status**: Research Complete - Ready for Decision  
**Full Research**: See `REBUILD_RECOVERY_RESEARCH.md` (20 parts, comprehensive)

---

## 🎯 Quick Answers to Your Questions

### Q1: Terminology?

**Answer**: ✅ **"Rebuild"** (not "recover")

**Why**:
- Industry standard (mdadm, hardware RAID, ZFS)
- Clear meaning (rebuilding missing data on replacement drive/backend)
- Distinct from "recover" (which implies recovering from corruption/failure)
- Distinct from our "heal" (which we use for self-healing)

**Examples from RAID systems**:
- **mdadm**: "Rebuild Status: 34% complete"
- **ZFS**: "resilvering" (their term for rebuild)
- **Hardware RAID**: "Rebuild in progress"

---

### Q2: Command Structure?

**Answer**: ✅ **`rclone backend rebuild level3:`**

**Why**:
- Follows rclone's backend command pattern
- Used by other backends (b2, hasher, crypt)
- Supports options and arguments
- Familiar to rclone users

**Pattern**:
```go
// In level3.go init():
fs.Register(&fs.RegInfo{
    CommandHelp: commandHelp,  // Register rebuild command
})

// Implement fs.Commander interface:
func (f *Fs) Command(ctx, name, arg, opt) (out, err) {
    case "rebuild": return f.rebuildCommand(...)
}
```

---

### Q3: Replacement Process?

**Answer**: Your proposed flow is correct! ✅

**Steps**:
```bash
# 1. Create new backend (or prepare empty one)
rclone mkdir new-odd-backend:

# 2. Update rclone.conf
[mylevel3]
type = level3
even = s3:bucket-even
odd = new-odd-backend:        # ← Changed from old-failed-backend:
parity = s3:bucket-parity

# 3. Check what needs rebuild
rclone backend rebuild level3: -o check-only=true
# Shows: Odd backend missing 1,247 files (2.3 GB)

# 4. Run rebuild
rclone backend rebuild level3: odd
# Rebuilds all particles (15 minutes)

# 5. Verify (optional)
rclone backend verify level3:
# Shows: All backends HEALTHY ✅

# 6. Test operations
rclone copy /tmp/test.txt level3:
# Writes work again! ✅
```

**No need to add another remote** - just replace existing one in config.

---

### Q4: Sub-commands vs Options?

**Answer**: ✅ **Use OPTIONS, not sub-commands**

**Recommended**:
```bash
# Analysis
rclone backend rebuild level3: -o check-only=true

# Rebuild with priority
rclone backend rebuild level3: odd -o priority=small

# Dry-run
rclone backend rebuild level3: odd -o dry-run=true
```

**Why options?**:
- ✅ Standard rclone pattern
- ✅ More flexible (can combine)
- ✅ Easier to maintain
- ✅ Consistent with other backends

**Not sub-commands like**:
```bash
rclone backend rebuild-check level3:    # ❌ Non-standard
rclone backend rebuild-dirs level3:     # ❌ Too many commands
```

---

## 🎨 Proposed Feature Set

### MVP (Phase 1) - Essential:

```bash
# Basic rebuild
rclone backend rebuild level3: odd

# Check what needs rebuild
rclone backend rebuild level3: -o check-only=true
```

**Features**:
- Rebuild specified backend (even/odd/parity)
- Auto-detect if backend not specified
- Progress display
- Basic verification

**Complexity**: ~200 lines, 4-6 hours

---

### Phase 2 - Advanced Options:

```bash
# Priority-based rebuild
rclone backend rebuild level3: odd -o priority=dirs
rclone backend rebuild level3: odd -o priority=small
rclone backend rebuild level3: odd -o priority=large

# Size filtering
rclone backend rebuild level3: odd -o max-size=100M

# Dry-run
rclone backend rebuild level3: odd -o dry-run=true

# Parallel uploads
rclone backend rebuild level3: odd -o parallel=10
```

**Complexity**: +150 lines, 2-4 hours

---

### Phase 3 - Verification:

```bash
# Separate verify command
rclone backend verify level3:
```

**Features**:
- Check particle sizes
- Verify parity calculation (spot check)
- Detect missing particles
- Health status report

**Complexity**: ~100 lines, 1-2 hours

---

## 📊 Rebuild vs Self-Healing

| Aspect | Self-Healing | Rebuild |
|--------|--------------|---------|
| **Trigger** | Automatic (read) | Manual (command) |
| **Scope** | Single file | All files |
| **Speed** | Background/slow | Foreground/fast |
| **Complete** | Only accessed files | Complete backend |
| **Use case** | Normal operations | Backend replacement |

**Both needed!** Different purposes:
- **Self-healing**: Day-to-day resilience
- **Rebuild**: Recovery from total backend loss

---

## 🚀 Recommended Implementation Order

### Now (Decision Phase):
1. ✅ Research complete (done!)
2. ✅ Question captured in OPEN_QUESTIONS.md (Q4)
3. ⏳ **Decision needed**: Implement MVP now or later?

### If Implementing MVP:
1. Add `fs.Commander` interface to Fs
2. Register `commandHelp` in `init()`
3. Implement basic `rebuild` command
4. Add progress tracking
5. Write tests
6. Document in README

### Later (Phase 2/3):
- Add advanced options (priority, filtering)
- Add verify command
- Add resume support
- Add detailed progress API

---

## 💡 Key Insights from Research

### 1. Standard Terminology
- ✅ **"Rebuild"** is the industry-standard RAID term
- Used by mdadm, hardware RAID, ZFS (resilver)
- Clear and familiar to users

### 2. Rclone Backend Commands
- ✅ Well-established pattern in rclone
- Examples: b2 (`cleanup`), hasher (`import`), crypt (`encode`)
- Easy to implement (`fs.Commander` interface)

### 3. Process Design
- ✅ Manual command is better than automatic (more control)
- ✅ Options better than sub-commands (standard pattern)
- ✅ Check-only mode essential for safety

### 4. Reuse Existing Code
- ✅ Can reuse `SplitBytes()`, `MergeBytes()`, `CalculateParity()`
- ✅ Similar to self-healing logic
- ✅ Not starting from scratch

### 5. Critical for Production
- ✅ Currently impossible to fully restore after backend replacement
- ✅ Self-healing only heals accessed files
- ✅ Rebuild needed for complete restoration

---

## 🎯 Recommendation

**Should implement**:  
✅ **Yes - Medium-High Priority**

**Why**:
- Critical gap in current functionality
- Users will need this in production
- Relatively straightforward to implement (reuses existing code)
- Standard RAID feature (expected by users)

**When**:
- **MVP now**: Basic rebuild + check-only mode
- **Phase 2 later**: Advanced options (priority, filtering)
- **Phase 3 future**: Verify command, resume support

**Estimated effort**:
- MVP: 4-6 hours
- Phase 2: +2-4 hours
- Phase 3: +1-2 hours

---

## 📚 Full Documentation

**See**: `docs/REBUILD_RECOVERY_RESEARCH.md`

**Contains** (20 parts):
1. Terminology research
2. Rclone backend command system
3. Process design
4. Command design
5. Detailed rebuild process
6. Sub-commands vs options
7. Auto-detect vs manual
8. Rebuild vs self-healing
9. Safety considerations
10. Command help text
11. Implementation strategy
12. Progress tracking
13. Verification design
14. Examples from other backends
15. Hardware RAID comparison
16. Answer to each of your questions
17. Implementation complexity estimates
18. Phased approach recommendations
19. Code examples
20. Reference materials

---

**Your question is now fully researched and documented!** 🎯

**Next**: Decide if/when to implement (tracked in OPEN_QUESTIONS.md Q4)

