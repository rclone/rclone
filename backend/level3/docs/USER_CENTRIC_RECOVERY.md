# User-Centric Recovery Design - Level3 Backend

**Date**: November 2, 2025  
**Focus**: How users discover, diagnose, and recover from backend failures  
**User Problem**: "My uploads don't work - what's wrong and how do I fix it?"

---

## 🎯 The User Experience Problem

### Current (Poor) UX:

```
User: $ rclone copy file.txt level3:
      ERROR: write blocked in degraded mode (RAID 3 policy): odd backend unavailable

User: "What does that mean? What do I do?"
User: *Googles error message*
User: *Reads documentation*
User: *Discovers backend commands exist*
User: $ rclone backend rebuild level3:
```

**Problems**:
- ❌ User doesn't know what "degraded mode" means
- ❌ Error doesn't explain HOW to fix it
- ❌ User must discover `backend` commands
- ❌ Multi-step process with no guidance
- ❌ Assumes technical knowledge of RAID

---

### Desired (Good) UX:

```
User: $ rclone copy file.txt level3:
      ERROR: Cannot upload to level3: odd backend unavailable
      
      Your level3 backend is in DEGRADED MODE:
        ✅ Even backend:   Available (1,247 files)
        ❌ Odd backend:    UNAVAILABLE (connection failed)
        ✅ Parity backend: Available (1,247 files)
      
      What this means:
        • Reads will work (using parity reconstruction)
        • Writes are blocked (RAID 3 safety policy)
        • You need to fix or replace the odd backend
      
      Next steps:
        1. Check if odd backend is temporarily down:
           $ rclone ls odd-backend:
        
        2. If permanently failed, replace in config and rebuild:
           a. Edit ~/.config/rclone/rclone.conf
           b. Replace: odd = old-failed-backend
              With:    odd = new-empty-backend
           c. Run: rclone check level3:
           d. Follow the instructions shown
      
      For more info: rclone help level3

User: "Ah, I understand! Let me check the backend..."
```

**Benefits**:
- ✅ Explains what's wrong in plain language
- ✅ Shows current status (visual)
- ✅ Provides actionable steps
- ✅ Guides through the recovery process
- ✅ No RAID jargon required

---

## 💡 User-Centric Solutions

### Solution 1: Enhanced Error Messages ⭐ **RECOMMENDED**

**Implementation**: Improve error messages in `Put()`, `Update()`, `Move()`

**Current Error**:
```
ERROR: write blocked in degraded mode (RAID 3 policy): odd backend unavailable
```

**Enhanced Error**:
```go
return fmt.Errorf(`cannot upload: level3 backend is DEGRADED

Status:
  ✅ Even backend:   Available
  ❌ Odd backend:    UNAVAILABLE (%w)
  ✅ Parity backend: Available

Reads work (using parity), but writes are blocked for data safety.

Fix:
  1. If odd backend is temporarily down, restart it
  2. If permanently failed, replace it:
     a. Edit your rclone config
     b. Replace odd backend with new empty backend
     c. Run: rclone check level3:
     d. Follow rebuild instructions

See: rclone help level3`, backendErr)
```

**Benefits**:
- ✅ No new commands needed
- ✅ Immediate guidance at point of failure
- ✅ Simple to implement (~50 lines)
- ✅ Users get help automatically

**Cons**:
- ⚠️ Long error messages (but informative!)

---

### Solution 2: `rclone check` Integration ⭐ **HIGHLY RECOMMENDED**

**Idea**: Users already know `rclone check` - make it level3-aware!

**Usage**:
```bash
$ rclone check level3:

Checking level3 backend health...

Backend Status:
  ✅ Even backend (s3:bucket-even):     1,247 files, 1.15 GB
  ❌ Odd backend (s3:bucket-odd-dead):  UNAVAILABLE (connection refused)
  ✅ Parity backend (s3:bucket-parity): 1,247 files, 1.15 GB

Overall Status: ⚠️ DEGRADED MODE

Impact:
  ✅ Reads: Working (using parity reconstruction)
  ❌ Writes: Blocked (strict RAID 3 policy)

Required Action: Replace failed odd backend

Steps to recover:
  1. Create new backend:
     $ rclone mkdir new-odd-backend:
     $ rclone ls new-odd-backend:  # Verify accessible
  
  2. Edit config (~/.config/rclone/rclone.conf):
     [mylevel3]
     type = level3
     even = s3:bucket-even
     odd = new-odd-backend:      # ← Change this line
     parity = s3:bucket-parity
  
  3. Check new backend detected:
     $ rclone check level3:       # Should show new backend
  
  4. Rebuild missing particles:
     $ rclone check level3: --rebuild
     # Or: rclone backend rebuild level3: odd
  
  5. Verify recovery:
     $ rclone check level3:       # Should show HEALTHY
  
For more info: rclone help level3
```

**Implementation**: Override standard `check` behavior for level3

**Benefits**:
- ✅ **Familiar command** (`rclone check` is well-known)
- ✅ Natural discovery (users run check when confused)
- ✅ Provides complete diagnosis
- ✅ Step-by-step recovery guide
- ✅ Can include `--rebuild` flag for convenience

**Complexity**: Medium (~200 lines)

---

### Solution 3: Health Status Command ⭐ **GOOD ADDITION**

**Idea**: Simple status command that's easy to discover

**Usage**:
```bash
$ rclone check level3:
# Or
$ rclone ls level3: --verbose

2025/11/02 INFO: Level3 Backend Status: ⚠️ DEGRADED
  ✅ Even:   1,247 files (healthy)
  ❌ Odd:    Unavailable
  ✅ Parity: 1,247 files (healthy)

Reads work, writes blocked. Run 'rclone check level3:' for recovery guide.
```

**Auto-display** during operations:
- Show status in `--verbose` mode
- Show brief health at start of operations
- Guide users to `check` command

**Benefits**:
- ✅ Proactive status display
- ✅ Minimal intrusion
- ✅ Guides to detailed help

---

### Solution 4: Interactive Recovery Mode ⭐ **FUTURE ENHANCEMENT**

**Idea**: Guided wizard for recovery

**Usage**:
```bash
$ rclone recover level3:
# Or
$ rclone check level3: --interactive

Level3 Backend Recovery Wizard
===============================

Step 1/5: Detecting backend status...
  ✅ Even backend:   Available (1,247 files)
  ❌ Odd backend:    UNAVAILABLE
  ✅ Parity backend: Available (1,247 files)

Step 2/5: Diagnosis
  Problem: Odd backend is not responding
  Impact: Writes blocked, reads work
  
  Is the odd backend permanently failed? (y/n): y

Step 3/5: Backend Replacement
  You need to create a new backend to replace 's3:bucket-odd-dead'
  
  What is the new backend? (e.g., s3:bucket-odd-new): s3:my-new-bucket
  
  Verifying new backend... ✅ Accessible and empty

Step 4/5: Update Configuration
  I'll update your config file. OK to proceed? (y/n): y
  ✅ Config updated: odd = s3:my-new-bucket

Step 5/5: Rebuild
  Ready to rebuild 1,247 files (2.3 GB) to odd backend.
  Estimated time: 15 minutes
  
  Proceed with rebuild? (y/n): y
  
  Rebuilding... [========>   ] 45% (2m 30s remaining)
  
  ✅ Rebuild complete!
  ✅ Backend status: HEALTHY
  
All done! Your level3 backend is now fully operational.
```

**Benefits**:
- ✅ **Zero technical knowledge required**
- ✅ Guided step-by-step
- ✅ Explains each action
- ✅ Can't make mistakes

**Cons**:
- ⚠️ Most complex to implement
- ⚠️ Interactive (not suitable for scripts)

**Use case**: First-time users, non-technical users

---

## 🎨 Recommended Multi-Layered Approach

### Layer 1: Enhanced Error Messages (Immediate)

**When writes fail**, show helpful error:

```go
if err := f.checkAllBackendsAvailable(ctx); err != nil {
    return nil, formatDegradedModeError(f, err)
}

func formatDegradedModeError(f *Fs, backendErr error) error {
    status := f.getBackendStatus(ctx)
    
    return fmt.Errorf(`Level3 backend is DEGRADED - writes blocked

Current Status:
  %s Even:   %s
  %s Odd:    %s
  %s Parity: %s

• Reads work (using parity reconstruction)
• Writes blocked (RAID 3 safety - prevents corruption)

To diagnose: rclone check level3:
For help: rclone help level3

Error: %w`, 
        status.evenIcon, status.evenStatus,
        status.oddIcon, status.oddStatus,
        status.parityIcon, status.parityStatus,
        backendErr)
}
```

**Benefits**: 
- ✅ Immediate help at point of failure
- ✅ Simple to implement
- ✅ No new commands needed

---

### Layer 2: Smart `rclone check` Command (High Priority)

**Override standard check** for level3:

```bash
$ rclone check level3:
```

**Shows**:
1. Backend health status (with icons ✅❌)
2. Impact assessment (what works, what doesn't)
3. Diagnosis (why it failed)
4. Step-by-step recovery guide
5. Commands to run next

**Add `--rebuild` flag**:
```bash
$ rclone check level3: --rebuild
# Performs: check → prompt user → rebuild if confirmed
```

**Benefits**:
- ✅ **Familiar command** (users know `rclone check`)
- ✅ One-stop diagnosis and recovery
- ✅ Natural workflow

---

### Layer 3: Verbose Mode Auto-Display (Low Priority)

**Auto-show status** in verbose mode:

```bash
$ rclone copy file.txt level3: -v

2025/11/02 INFO: Level3 Backend: ⚠️ DEGRADED (odd unavailable)
2025/11/02 INFO: Reads: ✅ Available  Writes: ❌ Blocked
2025/11/02 INFO: Run 'rclone check level3:' to diagnose
...
```

**Benefits**:
- ✅ Proactive notification
- ✅ Minimal intrusion
- ✅ Directs to help

---

### Layer 4: Backend Command (for advanced users)

**Keep** `rclone backend rebuild` for:
- Script automation
- Advanced users
- Fine-grained control

```bash
$ rclone backend rebuild level3: odd -o priority=small
```

**Benefits**:
- ✅ Scriptable
- ✅ Precise control
- ✅ Advanced options

---

## 🔄 Complete User Journey (Multi-Layer)

### Scenario: Odd Backend Permanently Failed

**Journey 1: Via Error Message** (Layer 1)
```
User: $ rclone copy file.txt level3:

System: ERROR: Level3 backend is DEGRADED
        ❌ Odd: UNAVAILABLE
        To diagnose: rclone check level3:
        
User: $ rclone check level3:

System: [Full diagnosis and recovery guide]
        Step 1: Create new backend...
        Step 2: Edit config...
        Step 3: Run rebuild...
        
User: [Follows steps]

System: ✅ Rebuild complete! Backend HEALTHY
```

---

**Journey 2: Proactive Check** (Layer 2)
```
User: "Hmm, uploads seem slow..."
User: $ rclone check level3:

System: ⚠️ DEGRADED MODE detected
        Odd backend: UNAVAILABLE
        [Recovery guide shown]
        
User: [Follows steps]
```

---

**Journey 3: Experienced User** (Layer 4)
```
User: [Already knows about backend failure]
User: $ rclone backend rebuild level3: odd
System: Rebuilding... ✅ Done
```

---

## 📋 Detailed Design: Enhanced `rclone check`

### Implementation:

```go
// In level3.go - Add to Features
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
    // Could override to show health status
    // But About is for storage usage, not health
}

// Better: Custom check implementation
// Rclone doesn't have a standard "Check" interface
// So we use backend command:

var commandHelp = []fs.CommandHelp{{
    Name:  "status",
    Short: "Show backend health and status",
    Long: `Shows the health status of all three backends...`,
}, {
    Name:  "rebuild",
    Short: "Rebuild missing particles after backend replacement",
    Long: `Rebuilds all missing particles...`,
}}
```

**But wait!** `rclone check` is for comparing two remotes, not health checks.

**Better approach**: Use a **common command** that users know:

---

## 💡 Alternative: Use `rclone ls` with Special Flag

**Pattern**: MinIO uses `mc admin info` for health

**For rclone**: Could use `rclone about` or create custom

Actually, let me reconsider...

---

## 🎯 BEST Solution: Multi-Pronged Approach

### 1. Enhanced Error Messages (Do First) ⭐

**In every write failure**, show:

```
ERROR: Cannot write to level3 backend (degraded mode)

Backend Health:
  ✅ even:   Available
  ❌ odd:    Connection refused (s3:bucket-odd)
  ✅ parity: Available

This means:
  • Reads work (using even + parity)
  • Writes blocked (prevents data corruption)
  
If odd backend is temporarily down:
  → Fix the backend and retry
  
If odd backend is permanently failed:
  → Run: rclone backend status level3:
  → This will guide you through recovery

See: https://rclone.org/level3/#error-handling
```

**Benefits**:
- ✅ Self-documenting errors
- ✅ Immediate help
- ✅ Points to next step
- ✅ URL for full docs

---

### 2. Add `status` Backend Command (Easy to Discover)

**Users can run**:
```bash
$ rclone backend status level3:
```

**Output**:
```
Level3 Backend Health Status
=============================

Backend Health:
  ✅ Even (s3:bucket-even):     1,247 files, 1.15 GB - HEALTHY
  ❌ Odd (s3:bucket-odd-dead):  UNAVAILABLE (last error: connection refused)
  ✅ Parity (s3:bucket-parity): 1,247 files, 1.15 GB - HEALTHY

Overall Status: ⚠️ DEGRADED MODE

What This Means:
  • Reads: ✅ Working (automatic parity reconstruction)
  • Writes: ❌ Blocked (RAID 3 data safety policy)
  • Self-healing: ⚠️ Cannot restore (odd backend unavailable)

Impact:
  • You can download/list/delete files
  • You cannot upload/modify/move files
  • Backend needs to be replaced for full operation

Recovery Steps:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Option A: Temporary Failure (Backend Will Come Back)
  1. Fix the backend (restart service, fix network, etc.)
  2. Verify: rclone ls s3:bucket-odd-dead
  3. Retry your operation

Option B: Permanent Failure (Backend Lost Forever)
  1. Create new backend:
     $ rclone mkdir new-odd-backend:
     $ rclone ls new-odd-backend:    # Verify it works
  
  2. Update config (~/.config/rclone/rclone.conf):
     [mylevel3]
     type = level3
     even = s3:bucket-even
     odd = new-odd-backend:          # ← Change this line
     parity = s3:bucket-parity
  
  3. Verify config updated:
     $ rclone backend status level3:
     # Should show new backend name
  
  4. Rebuild missing particles (estimated 15 minutes for 2.3 GB):
     $ rclone backend rebuild level3:
     # Or: rclone backend rebuild level3: odd
  
  5. Confirm recovery:
     $ rclone backend status level3:
     # Should show: HEALTHY ✅
  
  6. Test operations:
     $ rclone copy /tmp/test.txt level3:  # Should work!

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

For more information:
  • Documentation: rclone help level3
  • Degraded mode: See README.md section on Error Handling
  • Rebuild options: rclone backend help level3
```

**Benefits**:
- ✅ **Single command** to diagnose
- ✅ **Complete guide** shown immediately
- ✅ **Step-by-step** instructions
- ✅ **No guessing** what to do
- ✅ **Covers both scenarios** (temporary vs permanent)

---

### 3. Add `rebuild` as Shortcut Flag to Status

**Make it even easier**:
```bash
# Diagnose
$ rclone backend status level3:
# Shows status + recovery guide

# Rebuild (after following steps 1-3)
$ rclone backend status level3: --rebuild
# Synonym for: rclone backend rebuild level3:
```

**Or even better**:
```bash
$ rclone backend status level3: --fix

Detected: Odd backend needs rebuild
Missing: 1,247 files (2.3 GB)
Source: Even + parity (reconstruction)

Proceed with rebuild? (y/n): y

Rebuilding...
[==========> ] 50% (623/1,247 files)
```

**Benefits**:
- ✅ **Single command** for everything
- ✅ **Interactive confirmation**
- ✅ **Safest for users**

---

## 🎨 Implementation Priority

### Phase 1: Enhanced Error Messages (IMMEDIATE)

**Effort**: ~50 lines, 1 hour

**Impact**: High (every user benefits)

**Implementation**:
```go
// In checkAllBackendsAvailable():
func (f *Fs) checkAllBackendsAvailable(ctx context.Context) error {
    // ... check logic ...
    
    if err != nil {
        return f.formatDegradedError(ctx, failedBackend, err)
    }
}

func (f *Fs) formatDegradedError(ctx context.Context, failed string, err error) error {
    // Build multi-line helpful error
    // Include status, explanation, next steps
}
```

---

### Phase 2: `status` Backend Command (HIGH PRIORITY)

**Effort**: ~200 lines, 3-4 hours

**Impact**: High (primary diagnostic tool)

**Implementation**:
```go
var commandHelp = []fs.CommandHelp{{
    Name:  "status",
    Short: "Show backend health and recovery guide",
    Long: `...`,
}, {
    Name: "rebuild",
    Short: "Rebuild missing particles",
    Long: `...`,
}}

func (f *Fs) Command(ctx, name, arg, opt) {
    case "status": return f.statusCommand(ctx, opt)
    case "rebuild": return f.rebuildCommand(ctx, arg, opt)
}
```

---

### Phase 3: `rebuild` Command (MEDIUM PRIORITY)

**Effort**: ~200 lines, 4-6 hours

**Impact**: Medium (needed after diagnosis)

**Already designed**: See `REBUILD_RECOVERY_RESEARCH.md`

---

### Phase 4: Interactive Mode (FUTURE)

**Effort**: ~300 lines, 6-8 hours

**Impact**: Low (nice-to-have)

**For**: Non-technical users

---

## 📊 Comparison Table

| Approach | User Discovers How? | Commands Needed | UX Score | Complexity |
|----------|-------------------|-----------------|----------|------------|
| **Current** | Error → Google → Docs | Many | 2/10 | Low |
| **Enhanced Errors** | Automatic (in error) | None | 6/10 | Low |
| **Status Command** | Error → status command | 1-2 | 8/10 | Medium |
| **Check Integration** | rclone check (familiar) | 1 | 9/10 | Medium |
| **Interactive Wizard** | Recovery mode | 1 | 10/10 | High |

**Recommendation**: **Combine Enhanced Errors + Status Command** (Phases 1-2)

---

## 🎯 Proposed User Flow (Best UX)

### Step 1: User Encounters Error

```bash
$ rclone copy file.txt level3:

ERROR: Cannot write - level3 backend DEGRADED
  ❌ Odd backend unavailable
  
  Reads work, writes blocked (RAID 3 safety)
  
  Diagnose: rclone backend status level3:
  Help: rclone help level3
```

**User action**: Run status command (clear next step)

---

### Step 2: Diagnosis

```bash
$ rclone backend status level3:

[Shows comprehensive status + recovery guide]

Backend Health:
  ✅ Even: Available
  ❌ Odd: UNAVAILABLE
  ✅ Parity: Available

Recovery Guide:
  [Step-by-step instructions]
  1. Check if temporary failure...
  2. If permanent, create new backend...
  3. Update config...
  4. Run rebuild...
```

**User action**: Follow the guide (knows exactly what to do)

---

### Step 3: User Updates Config

```bash
# User edits ~/.config/rclone/rclone.conf
# Changes: odd = new-backend:
```

---

### Step 4: Rebuild

**Option A: Via status command**
```bash
$ rclone backend status level3: --rebuild
# Detects change, runs rebuild, shows progress
```

**Option B: Direct rebuild**
```bash
$ rclone backend rebuild level3:
# Auto-detects odd backend needs rebuild
```

**Option C: Explicit**
```bash
$ rclone backend rebuild level3: odd
```

---

### Step 5: Verification

```bash
$ rclone backend status level3:

Backend Health:
  ✅ Even: 1,247 files - HEALTHY
  ✅ Odd: 1,247 files - HEALTHY (rebuilt)
  ✅ Parity: 1,247 files - HEALTHY

Overall Status: ✅ HEALTHY

All operations available:
  ✅ Reads
  ✅ Writes
  ✅ Self-healing

Your level3 backend is fully operational! 🎉
```

---

## 💡 Key Insight: Make it Discoverable

### Problem with `rclone backend`:

**Users don't know**:
- What `backend` commands are
- How to discover them
- When to use them

**Solution**:
- ✅ Error messages guide to `backend status`
- ✅ `backend status` is memorable name
- ✅ Full recovery guide in one command
- ✅ No RAID knowledge required

### Comparison with Hardware RAID:

**Hardware RAID Controller**:
- Shows degraded status in BIOS/utility
- Guides user through replacement
- Automatic rebuild after replacement
- Progress display

**Level3 with Enhanced UX**:
- Shows degraded status in error + `status` command ✅
- Guides user through replacement ✅
- Manual rebuild (can't be automatic in rclone) ⚠️
- Progress display ✅

**Pretty close to hardware RAID UX!**

---

## 🎯 Final Recommendations

### Implement These (in order):

**1. Enhanced Error Messages** ✅ **Do First**
- Effort: 1 hour
- Impact: Huge
- Every failed write guides user

**2. `status` Backend Command** ✅ **Do Second**
- Effort: 3-4 hours
- Impact: Huge
- Central diagnostic tool

**3. `rebuild` Backend Command** ✅ **Do Third**
- Effort: 4-6 hours
- Impact: High
- Completes the workflow

**4. Interactive Mode** ⏸️ **Later**
- Effort: 6-8 hours
- Impact: Medium
- Nice-to-have

---

### Total Effort for Good UX:

**Minimum (Layers 1-2)**: 4-5 hours → **Great UX** ⭐  
**Complete (Layers 1-3)**: 8-11 hours → **Excellent UX** ⭐⭐  
**Full (All Layers)**: 14-19 hours → **Perfect UX** ⭐⭐⭐

---

## 📝 Update to OPEN_QUESTIONS.md

I'll update Q4 to include the user-centric approach!

**Key addition**: 
- `status` command (diagnostic + guide)
- Enhanced error messages (point to status)
- Multi-layer discovery (errors → status → rebuild)

---

**Your insight is excellent - users need guidance, not obscure commands!** 🎯

The combination of enhanced errors + status command provides excellent UX without requiring users to know about RAID internals!
