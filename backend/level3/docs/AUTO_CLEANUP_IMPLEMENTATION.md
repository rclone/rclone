# Level3 Auto-Cleanup Implementation Plan

**Date**: November 5, 2025  
**Feature**: Configurable auto-cleanup with explicit cleanup command  
**Status**: 🚀 **APPROVED** - Implementation in progress

---

## 🎯 Feature Overview

### Core Components

1. **`auto-cleanup` option** (default: `true`)
   - Controls whether broken objects (1 particle) are automatically hidden
   - Can be disabled for debugging/recovery scenarios

2. **`rclone cleanup` command support**
   - Explicitly removes broken objects across all remotes
   - Works on files, objects, directories, and buckets
   - Reports what was cleaned up

---

## ⚙️ Configuration

### Option Definition

```go
type Options struct {
    // ... existing options ...
    
    AutoCleanup bool `config:"auto_cleanup"`
}

var optionsConfig = fs.Options{{
    Name:     "auto_cleanup",
    Help:     "Automatically hide and clean up broken objects with only 1 particle",
    Default:  true,
    Advanced: false,
}}
```

### User Configuration

**Default Behavior** (auto-cleanup enabled):
```bash
$ rclone config create miniolevel3 level3 \
    even remote1: \
    odd remote2: \
    parity remote3:
# auto_cleanup defaults to true
```

**Debugging Mode** (auto-cleanup disabled):
```bash
$ rclone config create miniolevel3 level3 \
    even remote1: \
    odd remote2: \
    parity remote3: \
    auto_cleanup false
# Shows all objects including broken ones
```

---

## 🔧 Behavior by Mode

### Mode 1: Auto-Cleanup Enabled (Default)

| Operation | Behavior |
|-----------|----------|
| **List** | Show only objects with 2+ particles |
| **NewObject** | Require 2+ particles |
| **Remove** | Clean up ANY particles found (silent) |
| **Purge** | Delete all visible objects + cleanup fragments |
| **Cleanup** | Scan and remove all 1-particle objects |

**User Experience**:
```bash
$ rclone ls miniolevel3:mybucket
     1024 valid1.txt    # 2 particles
     2048 valid2.txt    # 3 particles
# broken.txt (1 particle) NOT shown

$ rclone purge miniolevel3:mybucket
# ✅ Works silently - no errors
```

---

### Mode 2: Auto-Cleanup Disabled (Debugging)

| Operation | Behavior |
|-----------|----------|
| **List** | Show ALL objects (including 1-particle) |
| **NewObject** | Require 2+ particles (but List shows all) |
| **Remove** | Clean up ANY particles found |
| **Purge** | Lists all objects, errors on broken ones |
| **Cleanup** | Scan and remove all 1-particle objects |

**User Experience**:
```bash
$ rclone ls miniolevel3:mybucket
     1024 valid1.txt    # 2 particles
     2048 valid2.txt    # 3 particles
      512 broken.txt    # 1 particle ⚠️

$ rclone cat miniolevel3:mybucket/broken.txt
ERROR: Cannot reconstruct broken.txt (only 1 particle present)

$ rclone cleanup miniolevel3:mybucket
Scanning mybucket for broken objects...
Found 1 broken object: broken.txt (1 particle)
Cleaned up 1 broken object

$ rclone ls miniolevel3:mybucket
     1024 valid1.txt
     2048 valid2.txt
# broken.txt now removed
```

**Why Useful**:
- 🔧 Debugging: See what went wrong
- 💾 Recovery: Manually copy broken objects if needed
- 🔍 Auditing: Verify system state
- 📊 Monitoring: Track broken object accumulation

---

## 🧹 Cleanup Command Implementation

### Interface

```go
// Cleanup removes broken objects (1 particle only)
// Implements optional Fs.Cleanup interface
func (f *Fs) Cleanup(ctx context.Context) error {
    // Scan all remotes for particles
    // Identify objects with only 1 particle
    // Remove those particles
    // Report what was cleaned up
}
```

### Behavior

**What It Cleans**:
- ✅ Files with only 1 particle (even, odd, or parity)
- ✅ Orphaned parity files (no corresponding data particles)
- ✅ Empty directories that became empty after cleanup
- ❌ Does NOT remove valid objects (2+ particles)
- ❌ Does NOT remove valid directories with contents

**Output**:
```bash
$ rclone cleanup miniolevel3:mybucket
Scanning mybucket for broken objects...
Found 5 broken objects:
  - file1.txt (1 particle in even: 512 bytes)
  - file2.txt (1 particle in odd: 1024 bytes)
  - file3.txt (1 particle in parity: 768 bytes)
  - dir/file4.txt (1 particle in even: 256 bytes)
  - dir/file5.txt (1 particle in parity: 512 bytes)
Cleaned up 5 broken objects (freed 3.0 KiB)

$ rclone cleanup miniolevel3:mybucket -vv
DEBUG : Scanning even remote...
DEBUG : Scanning odd remote...
DEBUG : Scanning parity remote...
DEBUG : Checking file1.txt: particles=[even] count=1 → BROKEN
DEBUG : Checking file2.txt: particles=[even,odd] count=2 → VALID
DEBUG : Removing even particle: file1.txt
INFO  : Cleaned up file1.txt (1 particle, 512 bytes)
...
```

---

## 📊 Implementation Details

### 1. Add Option to Backend

**File**: `backend/level3/level3.go`

```go
// Options for level3 backend
type Options struct {
    TimeoutMode string `config:"timeout_mode"`
    AutoCleanup bool   `config:"auto_cleanup"`
}

// Register fs with rclone
func init() {
    fs.Register(&fs.RegInfo{
        Name:        "level3",
        Description: "RAID 3 storage across three remotes",
        NewFs:       NewFs,
        Options: []fs.Option{{
            Name:     "even",
            Help:     "Remote for even-byte particles",
            Required: true,
        }, {
            Name:     "odd",
            Help:     "Remote for odd-byte particles",
            Required: true,
        }, {
            Name:     "parity",
            Help:     "Remote for parity particles",
            Required: true,
        }, {
            Name:     "timeout_mode",
            Help:     "Timeout mode for health checks",
            Default:  "balanced",
            Examples: []fs.OptionExample{
                {Value: "tolerant", Help: "10s timeouts"},
                {Value: "balanced", Help: "5s timeouts (default)"},
                {Value: "aggressive", Help: "2s timeouts"},
            },
        }, {
            Name:    "auto_cleanup",
            Help:    "Automatically hide and clean up broken objects (only 1 particle present)",
            Default: true,
            Advanced: false,
        }},
    })
}
```

---

### 2. Modify List() to Respect auto_cleanup

```go
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
    // ... existing logic to collect entries ...
    
    // Convert map back to slice
    entries = make(fs.DirEntries, 0, len(entryMap))
    for _, entry := range entryMap {
        switch e := entry.(type) {
        case fs.Object:
            // Check if auto-cleanup is enabled
            if f.opt.AutoCleanup {
                // Only include objects with 2+ particles
                particleCount := f.countParticlesSync(ctx, e.Remote())
                if particleCount < 2 {
                    fs.Debugf(f, "List: Skipping broken object %s (only %d particle)", 
                        e.Remote(), particleCount)
                    continue
                }
            }
            // Include object (either auto_cleanup off, or object is valid)
            entries = append(entries, &Object{
                fs:     f,
                remote: e.Remote(),
            })
        case fs.Directory:
            // Always include directories
            entries = append(entries, &Directory{
                fs:     f,
                remote: e.Remote(),
            })
        }
    }
    
    return entries, nil
}
```

---

### 3. Add Particle Counting Helper

```go
// countParticlesSync counts how many particles exist for an object
// Returns 0-3 (even, odd, parity)
func (f *Fs) countParticlesSync(ctx context.Context, remote string) int {
    count := 0
    
    // Check in parallel
    type result struct {
        name   string
        exists bool
    }
    resultCh := make(chan result, 3)
    
    go func() {
        _, err := f.even.NewObject(ctx, remote)
        resultCh <- result{"even", err == nil}
    }()
    
    go func() {
        _, err := f.odd.NewObject(ctx, remote)
        resultCh <- result{"odd", err == nil}
    }()
    
    go func() {
        // Check both parity suffixes
        _, errOL := f.parity.NewObject(ctx, GetParityFilename(remote, true))
        _, errEL := f.parity.NewObject(ctx, GetParityFilename(remote, false))
        resultCh <- result{"parity", errOL == nil || errEL == nil}
    }()
    
    // Collect results
    for i := 0; i < 3; i++ {
        res := <-resultCh
        if res.exists {
            count++
        }
    }
    
    return count
}

// particleInfo holds information about which particles exist
type particleInfo struct {
    remote       string
    evenExists   bool
    oddExists    bool
    parityExists bool
    count        int
}

// scanParticles scans a directory and returns particle information for all objects
func (f *Fs) scanParticles(ctx context.Context, dir string) ([]particleInfo, error) {
    // Collect all entries from all backends (without filtering)
    entriesEven, _ := f.even.List(ctx, dir)
    entriesOdd, _ := f.odd.List(ctx, dir)
    entriesParity, _ := f.parity.List(ctx, dir)
    
    // Build map of all unique object paths
    objectMap := make(map[string]*particleInfo)
    
    // Process even particles
    for _, entry := range entriesEven {
        if _, ok := entry.(fs.Object); ok {
            remote := entry.Remote()
            if objectMap[remote] == nil {
                objectMap[remote] = &particleInfo{remote: remote}
            }
            objectMap[remote].evenExists = true
        }
    }
    
    // Process odd particles
    for _, entry := range entriesOdd {
        if _, ok := entry.(fs.Object); ok {
            remote := entry.Remote()
            if objectMap[remote] == nil {
                objectMap[remote] = &particleInfo{remote: remote}
            }
            objectMap[remote].oddExists = true
        }
    }
    
    // Process parity particles
    for _, entry := range entriesParity {
        if _, ok := entry.(fs.Object); ok {
            remote := entry.Remote()
            // Strip parity suffix
            baseRemote, isParity, _ := StripParitySuffix(remote)
            if isParity {
                if objectMap[baseRemote] == nil {
                    objectMap[baseRemote] = &particleInfo{remote: baseRemote}
                }
                objectMap[baseRemote].parityExists = true
            }
        }
    }
    
    // Calculate counts
    result := make([]particleInfo, 0, len(objectMap))
    for _, info := range objectMap {
        if info.evenExists {
            info.count++
        }
        if info.oddExists {
            info.count++
        }
        if info.parityExists {
            info.count++
        }
        result = append(result, *info)
    }
    
    return result, nil
}
```

---

### 4. Implement Cleanup Command

```go
// Cleanup removes broken objects (only 1 particle present)
// This implements the optional Cleaner interface
func (f *Fs) Cleanup(ctx context.Context) error {
    fs.Infof(f, "Scanning for broken objects...")
    
    // Scan root directory recursively
    brokenObjects, totalSize, err := f.findBrokenObjects(ctx, "")
    if err != nil {
        return fmt.Errorf("failed to scan for broken objects: %w", err)
    }
    
    if len(brokenObjects) == 0 {
        fs.Infof(f, "No broken objects found")
        return nil
    }
    
    fs.Infof(f, "Found %d broken objects (total size: %s)", 
        len(brokenObjects), fs.SizeSuffix(totalSize))
    
    // Remove broken objects
    var cleanedCount int
    var cleanedSize int64
    for _, obj := range brokenObjects {
        fs.Infof(f, "Cleaning up broken object: %s (%d particle)", 
            obj.remote, obj.count)
        
        err := f.removeBrokenObject(ctx, obj)
        if err != nil {
            fs.Errorf(f, "Failed to clean up %s: %v", obj.remote, err)
            continue
        }
        
        cleanedCount++
        cleanedSize += obj.size
    }
    
    fs.Infof(f, "Cleaned up %d broken objects (freed %s)", 
        cleanedCount, fs.SizeSuffix(cleanedSize))
    
    return nil
}

// findBrokenObjects recursively finds all objects with only 1 particle
func (f *Fs) findBrokenObjects(ctx context.Context, dir string) ([]particleInfo, int64, error) {
    var brokenObjects []particleInfo
    var totalSize int64
    
    // Scan current directory
    particles, err := f.scanParticles(ctx, dir)
    if err != nil {
        return nil, 0, err
    }
    
    // Find broken objects (count == 1)
    for _, p := range particles {
        if p.count == 1 {
            // Get size of the single particle
            size := f.getBrokenObjectSize(ctx, p)
            p.size = size
            totalSize += size
            brokenObjects = append(brokenObjects, p)
        }
    }
    
    // Recursively scan subdirectories
    // List directories from all backends
    entries, err := f.listDirectories(ctx, dir)
    if err != nil {
        return brokenObjects, totalSize, err
    }
    
    for _, entry := range entries {
        if _, ok := entry.(fs.Directory); ok {
            subBroken, subSize, err := f.findBrokenObjects(ctx, entry.Remote())
            if err != nil {
                fs.Errorf(f, "Failed to scan directory %s: %v", entry.Remote(), err)
                continue
            }
            brokenObjects = append(brokenObjects, subBroken...)
            totalSize += subSize
        }
    }
    
    return brokenObjects, totalSize, nil
}

// getBrokenObjectSize gets the size of a broken object's single particle
func (f *Fs) getBrokenObjectSize(ctx context.Context, p particleInfo) int64 {
    if p.evenExists {
        obj, err := f.even.NewObject(ctx, p.remote)
        if err == nil {
            return obj.Size()
        }
    }
    if p.oddExists {
        obj, err := f.odd.NewObject(ctx, p.remote)
        if err == nil {
            return obj.Size()
        }
    }
    if p.parityExists {
        parityOL := GetParityFilename(p.remote, true)
        obj, err := f.parity.NewObject(ctx, parityOL)
        if err == nil {
            return obj.Size()
        }
        parityEL := GetParityFilename(p.remote, false)
        obj, err = f.parity.NewObject(ctx, parityEL)
        if err == nil {
            return obj.Size()
        }
    }
    return 0
}

// removeBrokenObject removes all particles of a broken object
func (f *Fs) removeBrokenObject(ctx context.Context, p particleInfo) error {
    g, gCtx := errgroup.WithContext(ctx)
    
    if p.evenExists {
        g.Go(func() error {
            obj, err := f.even.NewObject(gCtx, p.remote)
            if err != nil {
                return nil // Already gone
            }
            return obj.Remove(gCtx)
        })
    }
    
    if p.oddExists {
        g.Go(func() error {
            obj, err := f.odd.NewObject(gCtx, p.remote)
            if err != nil {
                return nil // Already gone
            }
            return obj.Remove(gCtx)
        })
    }
    
    if p.parityExists {
        g.Go(func() error {
            // Try both suffixes
            parityOL := GetParityFilename(p.remote, true)
            obj, err := f.parity.NewObject(gCtx, parityOL)
            if err == nil {
                return obj.Remove(gCtx)
            }
            parityEL := GetParityFilename(p.remote, false)
            obj, err = f.parity.NewObject(gCtx, parityEL)
            if err == nil {
                return obj.Remove(gCtx)
            }
            return nil // No parity found
        })
    }
    
    return g.Wait()
}

// listDirectories lists only directories (not objects) from a path
func (f *Fs) listDirectories(ctx context.Context, dir string) (fs.DirEntries, error) {
    // Use existing List but filter to directories only
    entries, err := f.List(ctx, dir)
    if err != nil {
        return nil, err
    }
    
    var dirs fs.DirEntries
    for _, entry := range entries {
        if _, ok := entry.(fs.Directory); ok {
            dirs = append(dirs, entry)
        }
    }
    
    return dirs, nil
}
```

---

### 5. Register Cleanup Interface

```go
// Check the interface is satisfied
var (
    _ fs.Fs       = (*Fs)(nil)
    _ fs.Purger   = (*Fs)(nil)
    _ fs.Mover    = (*Fs)(nil)
    _ fs.Cleaner  = (*Fs)(nil)  // NEW
)
```

---

## 📋 Testing Strategy

### Unit Tests

```go
func TestAutoCleanupEnabled(t *testing.T) {
    // Create level3 with auto_cleanup=true
    // Create valid object (3 particles)
    // Create broken object (1 particle)
    // List should show only valid object
    // Verify broken object not in list
}

func TestAutoCleanupDisabled(t *testing.T) {
    // Create level3 with auto_cleanup=false
    // Create valid object (3 particles)
    // Create broken object (1 particle)
    // List should show BOTH objects
    // NewObject on broken should fail
    // Remove on broken should succeed
}

func TestCleanupCommand(t *testing.T) {
    // Create level3 with auto_cleanup=false (to see broken objects)
    // Create 3 valid objects
    // Create 5 broken objects (1 particle each)
    // Run Cleanup()
    // Verify 5 broken objects removed
    // Verify 3 valid objects still present
}

func TestCleanupCommandRecursive(t *testing.T) {
    // Create directory structure:
    //   /dir1/file1.txt (valid)
    //   /dir1/file2.txt (broken)
    //   /dir2/file3.txt (broken)
    //   /dir2/subdir/file4.txt (valid)
    //   /dir2/subdir/file5.txt (broken)
    // Run Cleanup()
    // Verify only broken objects removed
    // Verify directory structure intact
}

func TestPurgeWithAutoCleanup(t *testing.T) {
    // Create level3 with auto_cleanup=true
    // Create valid + broken objects
    // Run Purge()
    // Verify no errors
    // Verify all particles removed (valid + broken)
}
```

---

## 📊 Performance Considerations

### Particle Counting Overhead

**Concern**: Counting particles for every object in List() could be slow

**Mitigations**:
1. **Parallel checking**: Use goroutines for even/odd/parity checks
2. **Short-circuit**: If auto_cleanup=false, skip counting entirely
3. **Batch checking**: Check multiple objects concurrently
4. **Cache results**: Within a single List() call, cache particle counts

**Benchmark Target**:
- List with 1000 objects should complete in <5 seconds
- Cleanup of 100 broken objects should complete in <10 seconds

---

## 📝 Documentation Updates

### 1. README.md

Add section:

```markdown
## Auto-Cleanup

By default, level3 automatically hides broken objects (only 1 particle present) from listings. This provides a clean user experience and prevents errors.

### Configuration

**Default behavior** (recommended):
```bash
rclone config create myremote level3 \
    even remote1: \
    odd remote2: \
    parity remote3:
# auto_cleanup defaults to true
```

**Debugging mode** (show all objects):
```bash
rclone config create myremote level3 \
    even remote1: \
    odd remote2: \
    parity remote3: \
    auto_cleanup false
# Shows broken objects in listings
```

### Cleanup Command

Remove broken objects explicitly:

```bash
$ rclone cleanup myremote:mybucket
Scanning mybucket for broken objects...
Found 5 broken objects
Cleaned up 5 broken objects (freed 3.0 KiB)
```

This is useful for:
- Cleaning up after backend failures
- Recovering from partial operations
- Periodic maintenance
```

### 2. OPEN_QUESTIONS.md

Update to mark as resolved:

```markdown
## ✅ RESOLVED: Handling of broken objects (1 particle)

**Resolution**: Implemented auto-cleanup option (default: true)
- Auto-cleanup enabled: Broken objects hidden and cleaned up automatically
- Auto-cleanup disabled: Broken objects visible for debugging
- Cleanup command: Explicit removal of broken objects
- See: AUTO_CLEANUP_IMPLEMENTATION.md
```

---

## ✅ Implementation Checklist

- [ ] Add `auto_cleanup` option to Options struct
- [ ] Add option to fs.Register config
- [ ] Implement `countParticlesSync()` helper
- [ ] Implement `scanParticles()` helper
- [ ] Modify `List()` to filter based on auto_cleanup
- [ ] Implement `Cleanup()` interface
- [ ] Implement `findBrokenObjects()` recursive scan
- [ ] Implement `removeBrokenObject()` cleanup
- [ ] Add Cleaner interface to type checks
- [ ] Write unit tests (5 test cases)
- [ ] Update README.md
- [ ] Update OPEN_QUESTIONS.md
- [ ] Run full test suite
- [ ] Manual testing with Minio

---

## 🎯 Expected Results

After implementation:

**Default Users** (auto-cleanup=true):
```bash
$ rclone ls miniolevel3:mybucket
     1024 file1.txt
     2048 file2.txt
# Clean, no broken objects shown

$ rclone purge miniolevel3:mybucket
# ✅ Works silently, no errors
```

**Power Users** (auto-cleanup=false):
```bash
$ rclone ls miniolevel3:mybucket
     1024 file1.txt
     2048 file2.txt
      512 broken.txt    # Broken object visible

$ rclone cleanup miniolevel3:mybucket
Cleaned up 1 broken object

$ rclone ls miniolevel3:mybucket
     1024 file1.txt
     2048 file2.txt
# Now clean
```

---

**Implementation ready to begin!** 🚀

