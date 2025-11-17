# Main Directory Documentation Cleanup Analysis

**Date**: 2025-11-16  
**Purpose**: Analyze documentation files in main `level3/` directory (not `docs/`)

---

## Current Files in Main Directory

1. **README.md** - ✅ **KEEP** - Essential user documentation
2. **RAID3.md** - ✅ **KEEP** - Technical specification
3. **TESTING.md** - ✅ **KEEP** - How to test (practical guide)
4. **TESTS.md** - ⚠️ **REVIEW** - Test overview (some redundancy)
5. **DESIGN_DECISIONS.md** - ✅ **KEEP** - Current design decisions
6. **OPEN_QUESTIONS.md** - ⚠️ **CLEAN UP** - Very long, has resolved questions
7. **EMAIL_DRAFT_NICK.md** - ⚠️ **MOVE/ARCHIVE** - Email draft (temporary)

---

## Analysis

### TESTS.md vs TESTING.md

**TESTS.md** (300 lines):
- Detailed test descriptions
- Test organization overview
- Test checklist
- References some outdated docs

**TESTING.md** (362 lines):
- How to run tests (practical)
- Manual testing instructions
- Bash harness documentation
- Test coverage overview

**Overlap**: Both cover test organization, but from different angles.

**Recommendation**: 
- **Option A**: Keep both (they serve different purposes)
- **Option B**: Merge TESTS.md content into TESTING.md and remove TESTS.md
- **Option C**: Trim TESTS.md to remove outdated references, keep as reference

**Suggested**: **Option C** - Keep both but clean up TESTS.md references

---

### OPEN_QUESTIONS.md

**Current State**: 890 lines, contains:
- 11 open questions (some resolved)
- 1 recently resolved question (Q11)
- Many questions with complete research but awaiting decisions
- Some questions superseded by others

**Issues**:
- Q11 is marked as resolved but still in "Recently Resolved" section
- Q6 is marked as "SUPERSEDED by Q4" but still present
- Many questions have complete research but are still "open"
- File is very long and hard to navigate

**Recommendation**: 
1. **Move resolved questions** to DESIGN_DECISIONS.md or remove
2. **Remove superseded questions** (Q6)
3. **Archive completed research** - Questions with complete research but awaiting decisions could be moved to a separate "RESEARCH_COMPLETE.md" file
4. **Keep only active open questions** in OPEN_QUESTIONS.md

**Suggested Action**: Clean up OPEN_QUESTIONS.md to contain only truly open questions needing decisions.

---

### EMAIL_DRAFT_NICK.md

**Current State**: Email draft for Nick Craig-Wood

**Recommendation**:
- **Option A**: Keep in main directory (temporary, might be useful reference)
- **Option B**: Move to `docs/` as `EMAIL_TO_NICK_CRAIG_WOOD.md` (archive)
- **Option C**: Remove after email is sent (temporary file)

**Suggested**: **Option B** - Move to docs/ for reference, or remove after sending

---

## Recommended Actions

### High Priority

1. **Clean up OPEN_QUESTIONS.md**:
   - Remove Q11 (resolved) - move to DESIGN_DECISIONS.md
   - Remove Q6 (superseded by Q4)
   - Consider moving questions with complete research to separate file
   - Keep only active questions needing decisions

2. **Update TESTS.md**:
   - Remove references to deleted docs
   - Update outdated information
   - Keep as reference document

### Medium Priority

3. **Move EMAIL_DRAFT_NICK.md**:
   - Move to `docs/EMAIL_TO_NICK_CRAIG_WOOD.md` or remove after sending

### Low Priority

4. **Consider consolidating TESTS.md and TESTING.md**:
   - Could merge into single comprehensive testing guide
   - But they serve different purposes, so keeping both is fine

---

## Files to Keep (Essential)

✅ **README.md** - User documentation  
✅ **RAID3.md** - Technical specification  
✅ **TESTING.md** - Testing guide  
✅ **DESIGN_DECISIONS.md** - Design decisions  
✅ **TESTS.md** - Test overview (after cleanup)  
✅ **OPEN_QUESTIONS.md** - Open questions (after cleanup)

---

## Summary

**Files to clean up**: 2-3 files
- OPEN_QUESTIONS.md (clean up, remove resolved)
- TESTS.md (update references)
- EMAIL_DRAFT_NICK.md (move or remove)

**No files to delete** - all serve a purpose, just need cleanup/updates.

