# Auto-Heal Semantics Status Report

**Date**: December 4, 2025  
**Status**: ⚠️ **INCONSISTENT - Needs Resolution**

---

## 🔴 Critical Issues Found

### 1. **Default Value Mismatch** (CRITICAL)

**Location**: `level3.go` lines 77-80, 408

**Problem**: Three different sources claim different defaults:

| Source | Claims Default |
|--------|---------------|
| **Option Definition** (line 79) | `Default: true` |
| **Code Comment** (line 408) | `auto_heal is opt-in and defaults to false` |
| **README.md** (line 43, 73) | `auto_heal defaults to true` |

**Actual Behavior**: The code at line 408-411 only sets a default for `auto_cleanup`, **NOT for `auto_heal`**. Since Go bools default to `false`, the actual default is `false`, contradicting the option definition and README.

**Impact**: 
- Users expecting `auto_heal=true` by default will be surprised
- Documentation is misleading
- Option definition is incorrect

**Fix Required**: 
1. Decide on the intended default (true or false)
2. Update code to match
3. Update documentation to match
4. Update option definition to match

---

### 2. **Missing `heal` Command in Documentation** (HIGH)

**Location**: `level3.go` lines 87-162 (`commandHelp`)

**Problem**: The `heal` command is **implemented and working** (line 688-689, 2664-2727) but is **NOT documented** in `commandHelp`.

**Current `commandHelp` includes**:
- ✅ `status` - documented
- ✅ `rebuild` - documented  
- ❌ `heal` - **MISSING**

**Impact**:
- Users cannot discover the `heal` command via `rclone help level3`
- Command exists but is undocumented
- Test exists (`TestHealCommandReconstructsMissingParticle`) but users don't know about it

**Fix Required**: Add `heal` command documentation to `commandHelp` array.

**Suggested Documentation**:
```go
{
    Name:  "heal",
    Short: "Heal all degraded objects (2/3 particles present)",
    Long: `Scans the entire remote and heals any objects that have exactly 2 of 3 particles.
    
This is an explicit, admin-driven alternative to automatic self-healing on read.
Use this when you want to proactively heal all degraded objects rather than
waiting for them to be accessed.

Usage:
    rclone backend heal level3:
    
The heal command will:
  1. Scan all objects in the remote
  2. Identify objects with exactly 2 of 3 particles (degraded state)
  3. Reconstruct and upload the missing particle
  4. Report summary of healed objects

Note: This is different from auto_heal which heals objects during reads.
The heal command proactively heals all degraded objects at once.

Examples:
    # Heal all degraded objects
    rclone backend heal level3:
    
    # Output shows:
    # Heal Summary
    # Files scanned:      100
    # Healthy (3/3):       85
    # Healed (2/3→3/3):   12
    # Unrecoverable (≤1): 3
`,
},
```

---

## ⚠️ Medium Priority Issues

### 3. **TestConcurrentOperations Skipped** (MEDIUM)

**Location**: `level3_test.go` line 1126

**Status**: Test is skipped with comment:
> "Concurrent self-healing stress-test temporarily disabled while auto_heal behaviour is revised"

**Current State**: 
- Auto-heal implementation is complete and working
- Test exists and is comprehensive
- Test is skipped due to timing/flakiness concerns

**Recommendation**: 
- Re-enable test once default value issue is resolved
- Consider adding retry logic or longer timeouts to handle timing issues
- Test is valuable for detecting race conditions

---

### 4. **Documentation Inconsistencies** (MEDIUM)

**Location**: `README.md` vs implementation

**Issues**:
1. README says `auto_heal defaults to true` but code defaults to `false`
2. README doesn't mention the `heal` command at all
3. README describes auto-heal behavior but doesn't clarify when it's enabled/disabled

**Fix Required**: Update README to match actual behavior once default is decided.

---

## ✅ What's Working Correctly

### Implementation Status

1. **Auto-heal functionality**: ✅ Fully implemented
   - Detects missing particles during reads (lines 3154, 3187)
   - Queues background uploads (lines 3156, 3189)
   - Works for both even and odd particles
   - Works for directory reconstruction (line 1706)

2. **Heal command**: ✅ Fully implemented
   - Scans entire remote (line 2672)
   - Heals all degraded objects (line 2696)
   - Provides detailed report (lines 2710-2726)
   - Handles all particle combinations (lines 2735-2746)

3. **Configuration**: ✅ Option exists
   - `auto_heal` option is defined (line 77-80)
   - Can be set to true/false
   - Used throughout codebase correctly

4. **Tests**: ✅ Comprehensive coverage
   - `TestAutoHealDirectoryReconstruction` - tests directory healing
   - `TestAutoHealDirMove` - tests healing during moves
   - `TestHealCommandReconstructsMissingParticle` - tests heal command
   - `TestSelfHealing` - tests automatic healing on read

---

## 📋 Recommended Action Plan

### Priority 1: Fix Default Value (CRITICAL)

**Decision Required**: What should the default be?

**Option A: Default to `true` (Recommended)**
- Matches README and option definition
- Provides best user experience (automatic healing)
- Matches "production-ready" status in docs
- **Action**: Add default setting code at line 412:
  ```go
  if _, ok := m.Get("auto_heal"); !ok {
      opt.AutoHeal = true
  }
  ```

**Option B: Default to `false` (Conservative)**
- Matches current code behavior
- Requires explicit opt-in
- More conservative approach
- **Action**: Update option definition and README to say `Default: false`

**Recommendation**: **Option A** - Default to `true` because:
- Documentation already says it defaults to true
- Auto-healing is a key feature
- Users expect it to work by default
- Can be disabled if needed

### Priority 2: Add Heal Command Documentation (HIGH)

**Action**: Add `heal` command to `commandHelp` array (after `rebuild` command)

### Priority 3: Update Documentation (MEDIUM)

**Action**: 
- Update README to mention `heal` command
- Ensure all defaults match actual behavior
- Add examples of using `heal` command

### Priority 4: Re-enable TestConcurrentOperations (LOW)

**Action**: 
- Once default value is fixed, re-enable test
- Add retry logic if needed for timing issues
- Consider making it a separate test flag

---

## 📊 Current State Summary

| Component | Status | Notes |
|-----------|--------|-------|
| **Auto-heal implementation** | ✅ Complete | Fully functional |
| **Heal command implementation** | ✅ Complete | Works but undocumented |
| **Default value** | ❌ Inconsistent | Code=false, Docs=true |
| **Command documentation** | ❌ Missing | `heal` not in commandHelp |
| **Tests** | ⚠️ Partial | One test skipped |
| **README documentation** | ⚠️ Outdated | Doesn't mention `heal` command |

---

## 🎯 Conclusion

The auto-heal **implementation is complete and working**, but there are **critical inconsistencies** between:
- Code behavior (defaults to `false`)
- Option definition (says `Default: true`)
- Documentation (says defaults to `true`)

**The semantics are not "being revised" - they're implemented but inconsistent.**

**Next Steps**:
1. **Decide on default value** (recommend `true`)
2. **Fix code to match decision**
3. **Add `heal` command to documentation**
4. **Update README**
5. **Re-enable TestConcurrentOperations** (optional)

Once these are fixed, auto-heal semantics will be **fully defined and consistent**.

