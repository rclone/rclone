# RAID 3 Rebuild/Recovery - Comprehensive Research

**Date**: November 2, 2025  
**Purpose**: Research rebuild/recovery process for level3 backend after backend replacement  
**Status**: Research & Design Phase

---

## 🎯 The Question

**Scenario**: One backend (e.g., "odd") fails permanently and is replaced with a new, empty backend.

**Goal**: Restore all missing particles to the new backend using the other two backends (even + parity).

**Questions**:
1. What is the correct terminology? (rebuild, recover, resync, heal)
2. How should the command work? (`rclone backend rebuild level3:`)
3. What should the process look like?
4. What sub-commands or options should we provide?

---

## 📚 Part 1: Terminology Research

### Industry Standard Terms:

| Term | Context | Meaning |
|------|---------|---------|
| **Rebuild** | RAID controllers, mdadm | Reconstruct data on replacement drive ⭐ |
| **Resync** | mdadm, MD RAID | Synchronize data across drives (used for rebuild) |
| **Resilver** | ZFS | ZFS-specific term for rebuild process |
| **Heal** | MinIO, GlusterFS | Check and repair inconsistencies |
| **Scrub** | ZFS, RAID | Read and verify all data (preventive) |
| **Recover** | General | Broader term (can mean data recovery from failure) |
| **Repair** | General | Fix corrupted or inconsistent data |

### Specific Command Examples:

**Linux mdadm** (Software RAID):
```bash
# Mark disk as failed
mdadm --manage /dev/md0 --fail /dev/sdb1

# Remove failed disk
mdadm --manage /dev/md0 --remove /dev/sdb1

# Add new disk (triggers automatic rebuild)
mdadm --manage /dev/md0 --add /dev/sdc1

# Monitor rebuild progress
mdadm --detail /dev/md0
# Shows: "Rebuild Status : 34% complete"
```

**Term used**: **"Rebuild"** (not "recover")

**ZFS**:
```bash
# Replace failed disk (triggers resilver)
zpool replace tank disk1 disk2

# Check resilver progress
zpool status tank
# Shows: "resilvering: 25.3% done"
```

**Term used**: **"Resilver"** (ZFS-specific, synonym for rebuild)

**MinIO**:
```bash
# Heal command (checks and repairs)
mc admin heal myminio

# Shows: "Heal status: objects scanned 1000, healed 50"
```

**Term used**: **"Heal"** (more like scrub + repair)

**Ceph**:
```bash
# Recovery happens automatically
# Monitor with:
ceph status
# Shows: "recovery: 45.2% complete"
```

**Term used**: **"Recovery"** (automatic process)

### Recommendation for Level3:

**Use "rebuild"** for these reasons:
- ✅ Industry standard for RAID systems (mdadm, hardware RAID)
- ✅ Clearly describes the operation (rebuilding missing data)
- ✅ Familiar to users with RAID experience
- ✅ Distinct from "recover" (which implies recovering from failure/corruption)

**Command name**: `rclone backend rebuild level3:`

**Why not other terms**:
- ❌ "recover" - Too generic, implies data was lost
- ❌ "resync" - Implies syncing existing data, not creating missing data
- ❌ "resilver" - ZFS-specific jargon
- ❌ "heal" - We already use for self-healing (different process)

---

## 🔧 Part 2: Rclone Backend Command System

### How Backend Commands Work:

**1. Registration** (in `init()` function):
```go
func init() {
    fs.Register(&fs.RegInfo{
        Name:        "level3",
        CommandHelp: commandHelp,  // Register commands
        // ...
    })
}

var commandHelp = []fs.CommandHelp{{
    Name:  "rebuild",
    Short: "Rebuild missing particles on a replacement backend",
    Long: `Detailed help text...`,
}}
```

**2. Implementation** (Command method):
```go
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out any, err error) {
    switch name {
    case "rebuild":
        return f.rebuildCommand(ctx, arg, opt)
    default:
        return nil, fs.ErrorCommandNotFound
    }
}
```

**3. Usage**:
```bash
rclone backend rebuild level3:
rclone backend rebuild level3: --option check
```

**4. Return Values**:
- `string` or `[]string` - Printed directly
- Other types - JSON encoded
- `nil` - No output (command succeeded)

### Examples from Other Backends:

**B2** - `cleanup`, `lifecycle` commands  
**Hasher** - `drop`, `dump`, `import` commands  
**Crypt** - `encode`, `decode` commands  
**Drive** - `shortcut`, `export-formats` commands  

**Pattern**: Commands are backend-specific utilities

---

## 🛠️ Part 3: Rebuild Process Design

### Hardware RAID 3 Rebuild Process:

1. **Replace failed drive** (physical swap)
2. **Controller detects new drive** (automatic)
3. **Rebuild starts automatically** (background)
4. **Monitor progress** (can check status)
5. **Rebuild completes** (can take hours)
6. **Array returns to healthy state**

**Key characteristics**:
- ⚡ **Automatic**: No manual command needed
- 📊 **Progress tracking**: Can monitor percentage complete
- ⏸️ **Can pause/resume**: Some controllers support this
- 🔄 **Continues after reboot**: Persistent state

### Level3 Rebuild Process (Proposed):

**Option A: Automatic (like Hardware RAID)**
```
User replaces remote in config → rclone detects → rebuild automatically
```
- ❌ Hard to implement (how to detect "new" backend?)
- ❌ May rebuild unnecessarily
- ✅ User-friendly (no manual command)

**Option B: Manual Command (Recommended)**
```
User replaces remote in config → User runs rebuild command → rebuild executes
```
- ✅ Simple to implement
- ✅ User has control
- ✅ Can check first (dry-run)
- ⚠️ Requires user action

**Recommendation**: **Option B (Manual)** - More controllable and explicit

---

## 🎨 Part 4: Rebuild Command Design

### Basic Command:

```bash
rclone backend rebuild level3: [backend-name]
```

**Arguments**:
- `[backend-name]` - Which backend to rebuild (even, odd, or parity)
- Optional: If omitted, auto-detect which backend(s) need rebuild

**Examples**:
```bash
# Rebuild odd backend (auto-detect missing particles)
rclone backend rebuild level3: odd

# Rebuild all missing particles (scan and rebuild)
rclone backend rebuild level3:

# Dry-run (check what would be rebuilt)
rclone backend rebuild level3: odd -o dry-run=true
```

---

### Sub-commands (Your Idea):

**Option A: Separate Commands**
```bash
rclone backend check level3:        # Analyze what needs rebuild
rclone backend rebuild level3: odd  # Perform rebuild
```

**Option B: Options/Flags**
```bash
rclone backend rebuild level3: odd -o check-only=true  # Check only
rclone backend rebuild level3: odd -o priority=small   # Prioritize small files
```

**Recommendation**: **Option B (Flags)** - Single command with options, standard rclone pattern

---

### Proposed Options:

```go
Options for rebuild command:

-o check-only=true    // Don't rebuild, just analyze
-o backend=odd        // Which backend to rebuild (even/odd/parity)
-o auto-detect=true   // Auto-detect missing backend (default)
-o priority=dirs      // Rebuild directories first
-o priority=small     // Rebuild small files first
-o priority=large     // Rebuild large files first
-o max-size=1G        // Only rebuild files smaller than this
-o min-size=1M        // Only rebuild files larger than this
-o dry-run=true       // Show what would be done
-o parallel=10        // Number of parallel rebuilds
```

---

## 📋 Part 5: Detailed Rebuild Process

### Step 1: User Replaces Backend in Config

**Before** (odd backend failed):
```ini
[mylevel3]
type = level3
even = s3:bucket-even
odd = s3:bucket-odd-FAILED     # This backend is dead
parity = s3:bucket-parity
```

**After** (user creates new backend):
```ini
[mylevel3]
type = level3
even = s3:bucket-even
odd = s3:bucket-odd-NEW        # New, empty backend
parity = s3:bucket-parity
```

---

### Step 2: Check What Needs Rebuild

```bash
$ rclone backend rebuild level3: -o check-only=true

Scanning level3 backend for missing particles...

Backend: odd
  Status: Missing particles detected
  Files affected: 1,247 files
  Directories: 42 directories
  Total size to rebuild: 2.3 GB
  
Can reconstruct from: even + parity
Rebuild time estimate: ~15 minutes (based on backend speed)

Ready to rebuild. Run without -o check-only=true to proceed.
```

---

### Step 3: Run Rebuild

```bash
$ rclone backend rebuild level3: odd

Rebuilding odd backend for level3:
  Source: even + parity (reconstruction)
  Target: odd backend
  
Progress:
[===================>        ] 65% (810/1247 files)
  Directories: 42/42 complete
  Small files (<1MB): 758/1050 complete
  Large files (>1MB): 52/197 complete
  Data transferred: 1.5 GB / 2.3 GB
  Speed: 8.5 MB/s
  ETA: 2 minutes
  
Errors: 0
```

---

### Step 4: Verification (Optional)

```bash
$ rclone backend verify level3:

Verifying level3 backend integrity...
  Checking particle sizes... ✅ All valid
  Checking parity consistency... ✅ All correct
  Checking for missing particles... ✅ None found
  
Backend status: HEALTHY
  Even: 1,247 files, 1.15 GB
  Odd: 1,247 files, 1.15 GB
  Parity: 1,247 files, 1.15 GB
  
All particles present and consistent!
```

---

## 💡 Part 6: Implementation Strategy

### Rebuild Algorithm:

```go
func (f *Fs) rebuildBackend(ctx context.Context, backendName string) error {
    // 1. Determine which backend to rebuild
    var targetBackend fs.Fs
    var sourceBackends [2]fs.Fs  // The two we'll use for reconstruction
    
    switch backendName {
    case "even":
        targetBackend = f.even
        sourceBackends = [2]fs.Fs{f.odd, f.parity}  // odd + parity → even
    case "odd":
        targetBackend = f.odd
        sourceBackends = [2]fs.Fs{f.even, f.parity} // even + parity → odd
    case "parity":
        targetBackend = f.parity
        sourceBackends = [2]fs.Fs{f.even, f.odd}    // even + odd → parity
    }
    
    // 2. List all files on source backends
    evenFiles, _ := operations.ListFn(ctx, f.even, ...)
    
    // 3. For each file:
    for _, file := range evenFiles {
        // Check if particle exists on target
        _, err := targetBackend.NewObject(ctx, file.Remote())
        if err == nil {
            continue // Already exists, skip
        }
        
        // Reconstruct particle from other two backends
        particle := reconstructParticle(ctx, file, sourceBackends, backendName)
        
        // Upload to target backend
        _, err = targetBackend.Put(ctx, particle, ...)
        
        // Track progress...
    }
    
    // 4. Return summary
    return nil
}
```

---

### Reconstruction Logic:

**We can reuse existing code!**

Current `Object.Open()` already does this:
- Detects missing particle
- Reconstructs from other two
- Returns the data

**For rebuild, we need**:
- Extract reconstructed data
- Upload to target backend
- This is similar to self-healing, but manual and complete

---

### Progress Tracking:

```go
type RebuildProgress struct {
    TotalFiles    int
    ProcessedFiles int
    TotalBytes    int64
    ProcessedBytes int64
    CurrentFile   string
    Errors        []string
    StartTime     time.Time
}

// Update progress periodically
// Return as JSON for monitoring
```

---

## 🔍 Part 7: Comparison with Existing Features

### vs. Self-Healing:

| Aspect | Self-Healing | Rebuild |
|--------|--------------|---------|
| **Trigger** | Automatic (during read) | Manual (user command) |
| **Scope** | Single file (as accessed) | All files in backend |
| **Speed** | Background (opportunistic) | Foreground (dedicated) |
| **Progress** | No tracking | Progress display |
| **When** | During normal operations | After backend replacement |

**Self-healing**: Opportunistic, gradual restoration  
**Rebuild**: Deliberate, complete restoration

**Both needed!** Different use cases.

---

### vs. rclone sync:

**Could user just use**:
```bash
rclone sync level3: new-odd-backend:
```

**Problems**:
- ❌ Would sync MERGED files, not particles
- ❌ Doesn't understand particle structure
- ❌ Can't rebuild from parity
- ❌ Wrong granularity

**Rebuild command needed**: Understands level3 particle structure

---

## 🎨 Part 8: Detailed Design Proposal

### Command Structure:

```bash
rclone backend rebuild level3: [backend] [options]
```

**Backend argument** (required or auto-detect):
- `even` - Rebuild even particles
- `odd` - Rebuild odd particles
- `parity` - Rebuild parity particles
- `auto` - Auto-detect which backend needs rebuild (default)

---

### Options (flags with `-o`):

```go
check-only=true|false        // Analyze only, don't rebuild (default: false)
dry-run=true|false           // Show what would be done (default: false)
backend=even|odd|parity|auto // Which backend to rebuild (default: auto)
parallel=N                   // Parallel uploads (default: 4)
priority=dirs|small|large    // Rebuild order (default: dirs)
max-size=SIZE                // Only files smaller than (e.g., 100M)
min-size=SIZE                // Only files larger than (e.g., 1K)
verify=true|false            // Verify after rebuild (default: true)
force=true|false             // Overwrite existing particles (default: false)
```

---

### Usage Examples:

**1. Check what needs rebuild** (dry-run):
```bash
$ rclone backend rebuild level3: -o check-only=true

Scanning level3: for missing particles...

Analysis Results:
  Even backend:   ✅ 1,247 files present
  Odd backend:    ❌ 0 files found (NEEDS REBUILD)
  Parity backend: ✅ 1,247 files present

Rebuild required for: odd backend
  Files to rebuild: 1,247
  Directories to create: 42
  Total size: 2.3 GB
  Estimated time: 15 minutes

Reconstruction method: even + parity → odd
  All files can be reconstructed ✅

Run without -o check-only=true to start rebuild.
```

---

**2. Rebuild odd backend**:
```bash
$ rclone backend rebuild level3: odd

Level3 Rebuild Starting...
  Target: odd backend (s3:bucket-odd-NEW)
  Source: even + parity (reconstruction)
  Mode: Full rebuild

Phase 1: Creating directories...
  [====================] 100% (42/42 directories)
  ✅ Directories created

Phase 2: Rebuilding particles...
  [============>       ] 60% (748/1,247 files)
  Current: large-video.mp4 (147 MB)
  Speed: 8.5 MB/s
  Transferred: 1.4 GB / 2.3 GB
  ETA: 2 minutes 15 seconds
  Errors: 0

Phase 3: Verification...
  [====================] 100% (1,247/1,247 files)
  ✅ All particles validated

Rebuild Complete!
  Files rebuilt: 1,247
  Directories created: 42
  Total size: 2.3 GB
  Duration: 14 minutes 32 seconds
  Average speed: 2.7 MB/s
  Errors: 0

Backend status: HEALTHY
  All three backends now have complete data ✅
```

---

**3. Rebuild with priority** (small files first):
```bash
$ rclone backend rebuild level3: odd -o priority=small

Rebuild Order: Small files first
  Files <1MB: 1,050 files (priority)
  Files >1MB: 197 files (after small files)
```

---

**4. Rebuild only specific size range**:
```bash
# Rebuild only small files for quick recovery
$ rclone backend rebuild level3: odd -o max-size=10M

Rebuilding files smaller than 10M...
  Files in range: 950 files
  Files skipped: 297 files (>10M)
```

---

**5. Verify without rebuild**:
```bash
$ rclone backend verify level3:

Checking particle consistency...
  Particle sizes: ✅ All valid
  Parity calculation: ✅ Checking 10 random files... OK
  Missing particles: ❌ Found 247 missing on odd backend

Backend status: DEGRADED
  Even: ✅ Complete
  Odd: ⚠️ Missing 247 files
  Parity: ✅ Complete

Recommendation: Run 'rclone backend rebuild level3: odd'
```

---

## 🔄 Part 9: Process Workflow

### Full Replacement Workflow:

```
1. Backend Failure Detected
   ↓
   User notices: reads work (degraded), writes fail

2. Prepare New Backend
   ↓
   $ rclone mkdir new-odd-backend:
   $ rclone ls new-odd-backend:  # Verify it's accessible

3. Update Config
   ↓
   Edit ~/.config/rclone/rclone.conf
   Replace: odd = old-failed-backend:
   With:    odd = new-odd-backend:

4. Verify Config
   ↓
   $ rclone backend features level3:
   # Should show new backend

5. Check Rebuild Requirements
   ↓
   $ rclone backend rebuild level3: -o check-only=true
   # Shows what needs to be done

6. Run Rebuild
   ↓
   $ rclone backend rebuild level3: odd
   # Rebuilds all missing particles

7. Verify Success
   ↓
   $ rclone backend verify level3:
   # Confirm all backends healthy

8. Test Operations
   ↓
   $ rclone copy /tmp/test.txt level3:
   $ rclone cat level3:test.txt
   # Verify writes and reads work
```

---

## 📊 Part 10: Rebuild Sub-commands vs Options

### Your Suggestion: Sub-commands

```bash
rclone backend rebuild-check level3:     # Check what needs rebuild
rclone backend rebuild-dirs level3:      # Rebuild directories only
rclone backend rebuild-small level3:     # Rebuild small files
rclone backend rebuild-all level3:       # Rebuild everything
```

**Pros**:
- ✅ Clear separation of operations
- ✅ Easy to understand
- ✅ Can have different help text

**Cons**:
- ⚠️ Many commands to maintain
- ⚠️ Non-standard for rclone (usually uses options)
- ⚠️ Harder to combine (can't do "check + rebuild")

---

### Alternative: Single Command with Options (Recommended)

```bash
rclone backend rebuild level3: odd -o check-only=true      # Check
rclone backend rebuild level3: odd -o priority=dirs        # Dirs first
rclone backend rebuild level3: odd -o priority=small       # Small first
rclone backend rebuild level3: odd                         # Everything
```

**Pros**:
- ✅ Single command to maintain
- ✅ Flexible combinations
- ✅ Standard rclone pattern
- ✅ Consistent with other backends

**Cons**:
- ⚠️ Slightly more complex options parsing

**Recommendation**: **Use options, not sub-commands**

---

## 🎯 Part 11: Recommended Implementation Plan

### Phase 1: Basic Rebuild (MVP)

**Commands**:
```bash
rclone backend rebuild level3: [even|odd|parity]
```

**Features**:
- ✅ Rebuild specified backend
- ✅ Reconstruct from other two backends
- ✅ Progress display
- ✅ Error handling
- ✅ Summary at end

**Implementation**:
- Reuse `SplitBytes()`, `MergeBytes()`, `CalculateParity()`
- Iterate through source backend
- Reconstruct missing particles
- Upload to target

**Complexity**: Medium (~200 lines)

---

### Phase 2: Analysis and Dry-Run

**Options added**:
```bash
-o check-only=true    # Analysis mode
-o dry-run=true       # Show what would be done
```

**Features**:
- ✅ Count files to rebuild
- ✅ Estimate size and time
- ✅ Identify which backend needs rebuild
- ✅ Check if reconstruction possible

**Complexity**: Low (~50 lines)

---

### Phase 3: Advanced Features

**Options added**:
```bash
-o priority=dirs|small|large   # Rebuild order
-o max-size=SIZE               # File filtering
-o parallel=N                  # Concurrency
-o verify=true                 # Post-rebuild verification
```

**Features**:
- ✅ Prioritize critical files
- ✅ Partial rebuild (by size)
- ✅ Faster rebuild (parallel)
- ✅ Integrity verification

**Complexity**: Medium (~150 lines)

---

### Phase 4: Verification Command

**Separate command**:
```bash
rclone backend verify level3:
```

**Features**:
- ✅ Check particle sizes
- ✅ Verify parity calculation (sample)
- ✅ Detect missing particles
- ✅ Report backend health

**Complexity**: Low (~100 lines)

---

## 🤔 Part 12: Auto-detect vs Manual Backend Specification

### Option A: Manual (Explicit)

```bash
rclone backend rebuild level3: odd    # User specifies
```

**Pros**:
- ✅ No ambiguity
- ✅ User has full control
- ✅ Simple to implement

**Cons**:
- ⚠️ User must know which backend failed
- ⚠️ Extra step (check first)

---

### Option B: Auto-detect (Smart)

```bash
rclone backend rebuild level3:       # Auto-detect missing backend
```

**How it works**:
```go
// Scan all three backends
evenCount := countParticles(f.even)
oddCount := countParticles(f.odd)
parityCount := countParticles(f.parity)

// Identify which has fewest files
if oddCount < evenCount && oddCount < parityCount {
    rebuildBackend = "odd"
} else if evenCount < oddCount && evenCount < parityCount {
    rebuildBackend = "even"
} else {
    rebuildBackend = "parity"
}
```

**Pros**:
- ✅ User-friendly (one command)
- ✅ Detects the problem automatically

**Cons**:
- ⚠️ May rebuild wrong backend if mismatched
- ⚠️ More complex logic

---

### Recommendation: **Support Both**

```bash
# Explicit (recommended for safety)
rclone backend rebuild level3: odd

# Auto-detect (convenience)
rclone backend rebuild level3:
# Prompts: "Detected odd backend needs rebuild. Proceed? (y/n)"
```

---

## ⚡ Part 13: Rebuild vs Self-Healing Comparison

### When to Use Each:

**Self-Healing** (Automatic):
- ✅ **During normal operations** (reads trigger healing)
- ✅ **Opportunistic** (heals what's accessed)
- ✅ **Background** (doesn't block operations)
- ⚠️ **Gradual** (may take days to heal all files)
- ⚠️ **Incomplete** (only heals accessed files)

**Rebuild** (Manual):
- ✅ **After backend replacement** (deliberate action)
- ✅ **Complete** (rebuilds ALL files)
- ✅ **Fast** (dedicated process)
- ✅ **Trackable** (progress display)
- ⚠️ **Manual** (requires user action)

**Example**:
```
Day 0: Odd backend fails
  → Self-healing handles read operations ✅
  → Writes fail (strict policy) ❌

Day 1: Replace odd backend
  → Run rebuild command
  → 15 minutes later: All particles restored
  → Writes work again ✅
```

---

## 🛡️ Part 14: Safety Considerations

### Rebuild Safety:

**1. Source Backend Availability**:
```go
// Before rebuild, verify source backends healthy
if err := f.checkBackendsAvailable(ctx, sourceBackends); err != nil {
    return fmt.Errorf("cannot rebuild: source backends unavailable: %w", err)
}
```

**2. Target Backend Connectivity**:
```go
// Verify target backend is writable
if err := targetBackend.Mkdir(ctx, ".rebuild-test"); err != nil {
    return fmt.Errorf("target backend not writable: %w", err)
}
```

**3. Overwrite Protection**:
```go
// By default, don't overwrite existing particles
_, err := targetBackend.NewObject(ctx, particle.Remote())
if err == nil && !force {
    skip("Particle exists, use -o force=true to overwrite")
}
```

**4. Atomic Operations**:
```go
// Upload to temporary name, then rename
tempName := particle.Remote() + ".rebuilding"
_, err := targetBackend.Put(ctx, data, tempInfo)
// Then rename to final name
```

**5. Resume Support** (Future):
```go
// Track completed files
// Allow resuming interrupted rebuild
// Useful for large datasets
```

---

## 📋 Part 15: Command Help Text

### Proposed CommandHelp:

```go
var commandHelp = []fs.CommandHelp{{
    Name:  "rebuild",
    Short: "Rebuild missing particles on a replacement backend",
    Long: `This command rebuilds all missing particles on a replacement backend.

Use this after replacing a failed backend with a new, empty backend. The rebuild
process reconstructs all missing particles using the other two backends and parity
information, restoring the level3 backend to a fully healthy state.

Usage Examples:

    # Check what needs to be rebuilt
    rclone backend rebuild level3: -o check-only=true
    
    # Rebuild odd backend (auto-detected as missing)
    rclone backend rebuild level3:
    
    # Rebuild specific backend
    rclone backend rebuild level3: odd
    
    # Rebuild with small files first
    rclone backend rebuild level3: odd -o priority=small
    
    # Dry-run to see what would be done
    rclone backend rebuild level3: odd -o dry-run=true

Options:

- check-only: Analyze what needs rebuild without actually rebuilding
- dry-run: Show what would be done without making changes
- backend: Specify which backend to rebuild (even, odd, parity, or auto)
- priority: Rebuild order (dirs, small, large)
- parallel: Number of concurrent rebuilds (default: 4)
- max-size: Only rebuild files smaller than this size
- verify: Verify particle integrity after rebuild (default: true)
- force: Overwrite existing particles (default: false)

The rebuild process:
1. Scans all files on the source backends
2. Identifies missing particles on target backend
3. Reconstructs missing data from other two backends
4. Uploads reconstructed particles to target backend
5. Verifies integrity (if verify=true)

Progress is displayed during rebuild, showing files processed, data transferred,
and estimated time remaining.
`,
}, {
    Name:  "verify",
    Short: "Verify backend integrity and consistency",
    Long: `This command verifies the integrity of the level3 backend.

It checks:
- Particle sizes are valid (even vs odd relationship)
- All particles are present
- Parity calculation is correct (spot checks)
- Backend health status

Usage Example:

    rclone backend verify level3:

Returns a health report showing the status of all three backends and any
issues detected.
`,
}}
```

---

## 🎯 Part 16: Answers to Your Questions

### Q1: Terminology?

**Answer**: **"Rebuild"** is the correct technical term.

**Rationale**:
- Standard RAID terminology (mdadm, hardware RAID)
- Clear meaning (rebuilding lost data)
- Distinct from "recover" (which implies recovering from corruption)
- Distinct from "heal" (which we use for automatic self-healing)

---

### Q2: Command Structure?

**Answer**: **Yes, `rclone backend rebuild level3:`**

**Rationale**:
- Follows rclone's backend command pattern
- Used by other backends (b2, hasher, crypt, etc.)
- Familiar to rclone users
- Supports options and arguments

**Full command**:
```bash
rclone backend rebuild level3: [backend] [options]
```

---

### Q3: Replacement Process?

**Answer**: Your proposed flow is correct, with slight modifications:

**Steps**:
1. ✅ Create new backend (or use existing empty one)
2. ✅ Update `rclone.conf` - replace failed backend with new one
3. ✅ Run check: `rclone backend rebuild level3: -o check-only=true`
4. ✅ Run rebuild: `rclone backend rebuild level3: odd`
5. ✅ Verify: `rclone backend verify level3:` (optional)
6. ✅ Test: Try some operations to confirm

**No need** to add another remote - just replace the existing one in config.

---

### Q4: Sub-commands?

**Answer**: **Use options, not sub-commands**

**Proposed options**:
- `-o check-only=true` - Analyze what needs rebuild
- `-o priority=dirs` - Rebuild directories first
- `-o priority=small` - Rebuild small files first
- `-o priority=large` - Rebuild large files first
- `-o dry-run=true` - Show what would be done
- `-o verify=true` - Verify after rebuild (default)
- `-o parallel=N` - Concurrent rebuilds

**Rationale**:
- Standard rclone pattern (options with `-o`)
- More flexible (can combine options)
- Easier to maintain (single command)
- Consistent with other backends

---

## 💡 Part 17: Additional Commands to Consider

### 1. `verify` Command

**Purpose**: Check backend health without rebuilding

```bash
rclone backend verify level3:
```

**Outputs**: Health report

---

### 2. `stats` Command

**Purpose**: Show backend statistics

```bash
rclone backend stats level3:

Even backend:   1,247 files, 1.15 GB
Odd backend:    1,247 files, 1.15 GB
Parity backend: 1,247 files, 1.15 GB
Total files: 1,247
Total redundant storage: 3.45 GB (original: 2.3 GB)
Storage efficiency: 150% (50% overhead)
```

---

### 3. `check` Command (Alias for rebuild with check-only)

**Purpose**: Shorthand for analysis

```bash
rclone backend check level3:
# Same as: rclone backend rebuild level3: -o check-only=true
```

---

## 🚀 Part 18: Implementation Complexity

### Effort Estimates:

| Component | Lines | Difficulty | Time |
|-----------|-------|------------|------|
| **Basic rebuild** | ~200 | Medium | 4-6 hours |
| **Progress tracking** | ~100 | Medium | 2-3 hours |
| **Check/analysis** | ~100 | Low | 1-2 hours |
| **Priority/filtering** | ~150 | Medium | 2-4 hours |
| **Verification** | ~100 | Low | 1-2 hours |
| **Documentation** | ~200 lines | Low | 1-2 hours |
| **Tests** | ~300 | Medium | 3-4 hours |
| **Total** | ~1,250 | Medium | **14-23 hours** |

**Phased approach**: Start with MVP (basic rebuild), add features iteratively

---

## ✅ Part 19: Recommended Approach

### Immediate (Document as Open Question):

Add to `OPEN_QUESTIONS.md` as Q8:

```markdown
### Q8: Rebuild Command for Backend Replacement
**Priority**: Medium
**Decision Needed**: Should we implement, and what features?
```

---

### Short-term (MVP Implementation):

**Implement**:
1. Basic `rebuild` command
2. Check-only mode
3. Progress display
4. Verification

**Skip for now**:
- Priority options (can add later)
- Size filtering (can add later)
- Resume support (future enhancement)

---

### Long-term (Full Feature Set):

**Add**:
- Priority-based rebuild
- Size-based filtering
- Resume support
- Detailed verification
- Stats command

---

## 📚 Part 20: References

### RAID Terminology:
- **mdadm**: Uses "rebuild" for replacing failed drives
- **ZFS**: Uses "resilver" (ZFS-specific term for rebuild)
- **Hardware RAID**: Uses "rebuild" (standard term)
- **Ceph**: Uses "recovery" (automatic process)
- **MinIO**: Uses "heal" (for checking and repairing)

### Rclone Patterns:
- Backend commands: `fs.RegInfo.CommandHelp`
- Command implementation: `fs.Commander` interface
- Examples: b2, hasher, crypt, drive

### Our Terminology:
- **Self-healing**: Automatic, opportunistic restoration during reads
- **Rebuild**: Manual, complete restoration after backend replacement
- **Verify**: Check health without modifications

---

## 🎯 Recommendation Summary

**Terminology**: ✅ **"Rebuild"** (standard RAID term)

**Command**: ✅ **`rclone backend rebuild level3: [backend]`**

**Process**:
1. User replaces failed backend in config
2. User runs: `rclone backend rebuild level3: -o check-only=true`
3. User runs: `rclone backend rebuild level3: odd`
4. System rebuilds all missing particles
5. User verifies: `rclone backend verify level3:`

**Options** (not sub-commands):
- `-o check-only=true` - Analysis
- `-o dry-run=true` - Preview
- `-o priority=dirs|small|large` - Order
- `-o parallel=N` - Speed
- `-o verify=true` - Safety

**Implementation**: Phased (MVP first, then enhance)

---

**Next Step**: Add to OPEN_QUESTIONS.md and decide if/when to implement!

